package controller

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"

	frpv1alpha1 "github.com/imneov/kube-frp/api/v1alpha1"
	kubefrp "github.com/imneov/kube-frp/pkg/util/kubernetes"
	"github.com/imneov/servicekeel/internal/dns"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// FRPSServerRouter uses a controller-runtime manager and reconciler to watch FRPServer CRs and return endpoints.
type FRPSServerRouter struct {
	mgr            ctrl.Manager
	client         ctrlclient.Client
	interval       time.Duration
	endpoints      []*EndpointInfo
	lock           sync.RWMutex
	mappedServices // 需要监控的services
	dnsServer      *dns.Server
}

// NewFRPSServer creates a new controller-runtime manager to watch FRPServer CRs.
func NewFRPSServer(services []string, dnsServer *dns.Server) (*FRPSServerRouter, error) {
	// 获取 kubeconfig 文件路径
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		home := os.Getenv("HOME")
		kubeconfig = filepath.Join(home, ".kube", "config")
	}
	if len(services) == 0 {
		return nil, fmt.Errorf("services is empty")
	}
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to build kubeconfig: %v", err)
	}
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(frpv1alpha1.AddToScheme(scheme))

	mgr, err := ctrl.NewManager(config, ctrl.Options{Scheme: scheme})
	if err != nil {
		return nil, fmt.Errorf("failed to start manager: %v", err)
	}

	watcher := &FRPSServerRouter{
		client:         mgr.GetClient(),
		mgr:            mgr,
		mappedServices: services,
	}
	if err := ctrl.NewControllerManagedBy(watcher.mgr).
		For(&frpv1alpha1.FRPServer{}).
		Complete(watcher); err != nil {
		return nil, fmt.Errorf("failed to setup controller: %v", err)
	}
	return watcher, nil
}

// Start starts the controller manager.
func (w *FRPSServerRouter) Start() {
	go func() {
		if err := w.mgr.Start(ctrl.SetupSignalHandler()); err != nil {
			log.Fatalf("manager stopped: %v", err)
		}
	}()
}

// Reconcile lists all FRPServer CRs and sends Router data on each event.
func (r *FRPSServerRouter) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var list frpv1alpha1.FRPServerList
	if err := r.client.List(ctx, &list); err != nil {
		log.Printf("failed to list FRPServer CRs: %v", err)
		return ctrl.Result{RequeueAfter: r.interval}, nil
	}

	// Flatten each active connection into a Router record
	endpoints := make([]*EndpointInfo, len(r.mappedServices))
	for _, item := range list.Items {
		// Determine the host address for FRP server: prefer ProxyBindAddr, fallback to BindAddr
		ServerAddr := item.Status.InternetAddr
		if ServerAddr == "" {
			ServerAddr = item.Spec.BindAddr
		}

		ServerPort := item.Spec.BindPort
		if ServerPort == 0 {
			ServerPort = 7000
		}

		for _, conn := range item.Status.ActiveConnections {
			sk := conn.ProxyConfig.SecretKey
			serviceName := conn.ProxyName
			info, err := kubefrp.ParseServiceName(serviceName)
			if err != nil {
				log.Printf("failed to parse service name: %v", err)
				continue
			}
			// 如果当前service不在需要监控的services列表中，则跳过
			idx := slices.Index(r.mappedServices, info.Name)
			if idx == -1 {
				continue
			}

			// 如果当前service在需要监控的services列表中，则添加到endpoints列表中
			endpoint := &EndpointInfo{
				ServiceName:     info.Name,
				ServiceProtocol: info.Protocol,
				ServicePort:     info.Port,
				ServerAddr:      ServerAddr,
				SecretKey:       sk,
				ServerPort:      fmt.Sprintf("%d", ServerPort),
			}
			endpoints = append(endpoints, endpoint)
		}
	}
	r.lock.Lock()
	r.endpoints = endpoints
	r.lock.Unlock()
	return ctrl.Result{RequeueAfter: r.interval}, nil
}

func (r *FRPSServerRouter) Endpoints() []*EndpointInfo {
	return r.endpoints
}
