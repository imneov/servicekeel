package controller

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"

	frpv1alpha1 "github.com/imneov/kube-frp/api/v1alpha1"
	kubefrp "github.com/imneov/kube-frp/pkg/util/kubernetes"
	"github.com/imneov/servicekeel/internal/dns"
	"github.com/prometheus/client_golang/prometheus"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

var once sync.Once

const (
	defaultInterval   = 10 * time.Second
	defaultServerPort = 7000
)

// FRPClientInterface defines an interface for starting and stopping FRP clients.
type FRPClientInterface interface {
	Start() error
	Stop() error
}

// Option configures FRPSServerController dependencies.
type Option func(*FRPSServerController)

// WithKubernetesClientFactory injects a custom Kubernetes client factory.
func WithKubernetesClientFactory(factory func() (ctrlclient.Client, error)) Option {
	return func(r *FRPSServerController) {
		r.getClient = factory
	}
}

// WithFRPClientFactory injects a custom FRP client factory.
func WithFRPClientFactory(factory func(name string, info *EndpointInfo) (FRPClientInterface, error)) Option {
	return func(r *FRPSServerController) {
		r.frpFactory = factory
	}
}

var (
	frpEndpointCount = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "servicekeel_frp_endpoint_count",
		Help: "Number of active FRP endpoints currently watched",
	})
	frpReconcileErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "servicekeel_frp_reconcile_errors_total",
		Help: "Total number of reconciliations that returned an error",
	})
)

func init() {
	prometheus.MustRegister(frpEndpointCount, frpReconcileErrors)
}

// FRPSServerController uses a controller-runtime manager and reconciler to watch FRPServer CRs and return endpoints.
type FRPSServerController struct {
	mappedServices map[string]struct{} // 需要监控的services
	client         ctrlclient.Client
	getClient      func() (ctrlclient.Client, error)
	frpFactory     func(name string, info *EndpointInfo) (FRPClientInterface, error)
	interval       time.Duration
	lock           sync.RWMutex
	dnsServer      *dns.Server
	endpoints      map[string]*EndpointInfo // 当前监控的endpoints，对应 dns 服务地址
}

// NewFRPSServer creates a new controller-runtime manager to watch FRPServer CRs.
func NewFRPSServer(services []string, dnsServer *dns.Server, opts ...Option) (*FRPSServerController, error) {
	if len(services) == 0 {
		return nil, fmt.Errorf("services is empty")
	}
	mappedServices := make(map[string]struct{}, len(services))
	for _, svc := range services {
		mappedServices[svc] = struct{}{}
	}

	r := &FRPSServerController{
		mappedServices: mappedServices,
		interval:       defaultInterval,
		dnsServer:      dnsServer,
		endpoints:      make(map[string]*EndpointInfo),
		getClient:      kubernetesClient,
		frpFactory:     func(name string, info *EndpointInfo) (FRPClientInterface, error) { return NewFRPClient(name, info) },
	}
	for _, opt := range opts {
		opt(r)
	}
	cli, err := r.getClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %v", err)
	}
	r.client = cli
	return r, nil
}

// Start starts periodic reconciliation and returns a channel of EndpointInfo slices.
func (r *FRPSServerController) Start() {
	once.Do(func() {
		go func() {
			defer utilruntime.HandleCrash()
			ticker := time.NewTicker(r.interval)
			defer ticker.Stop()
			logger := ctrl.Log.WithName("frps-controller")
			for range ticker.C {
				if _, err := r.Reconcile(context.Background(), ctrl.Request{}); err != nil {
					logger.Error(err, "failed to reconcile")
					frpReconcileErrors.Inc()
				}
			}
		}()
	})
}

func (r *FRPSServerController) GetEndpoint(proxyName string) *EndpointInfo {
	r.lock.RLock()
	defer r.lock.RUnlock()
	return r.endpoints[proxyName]
}

func (r *FRPSServerController) GetAllEndpoints() map[string]*EndpointInfo {
	r.lock.RLock()
	defer r.lock.RUnlock()
	return r.endpoints
}

// Reconcile lists all FRPServer CRs and sends Router data on each event.
func (r *FRPSServerController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := ctrl.Log.WithName("frps-controller")

	// 1. get all endpoints
	endpoints, err := r.getAllEndpoints(ctx)
	if err != nil {
		frpReconcileErrors.Inc()
		return ctrl.Result{RequeueAfter: r.interval}, err
	}

	// 2. 根据 endpoints 更新 frp 客户端池, 更新 dns 服务
	// 删除已经不存在的隧道
	for key, ep := range r.endpoints {
		if _, ok := endpoints[key]; !ok {
			// 停止 frp 客户端
			err := ep.FRPClient.Stop()
			if err != nil {
				logger.Error(err, "failed to stop frp client")
			}
			// 删除 dns 映射
			ip, err := r.dnsServer.RemoveMapping(ep.ServiceName)
			if err != nil {
				logger.Error(err, "failed to remove mapping")
			} else {
				logger.Info("removed mapping", "service", ep.ServiceName, "ip", ip)
			}
			// 删除 endpoints 元素
			delete(r.endpoints, key)
		}
	}

	// 新增新增的隧道
	for key, ep := range endpoints {
		if _, ok := r.endpoints[key]; !ok {
			// 新增 dns 映射，并获得 mappedIP 地址
			mappedIP, err := r.dnsServer.AddMapping(ep.ServiceName)
			if err != nil {
				logger.Error(err, "failed to add mapping")
				r.dnsServer.RemoveMapping(ep.ServiceName)
				continue
			} else {
				logger.Info("added mapping", "service", ep.ServiceName, "ip", mappedIP)
			}
			ep.MappedIP = mappedIP.String()
			// 新增 frp 客户端
			frpClient, err := r.frpFactory(ep.ProxyName, ep)
			if err != nil {
				logger.Error(err, "failed to create frp client")
				continue
			}
			err = frpClient.Start()
			if err != nil {
				logger.Error(err, "failed to start frp client")
				r.dnsServer.RemoveMapping(ep.ServiceName)
				continue
			} else {
				logger.V(4).Info("started frp client", "proxy", ep.ProxyName)
			}
			ep.FRPClient = frpClient
			r.endpoints[key] = ep
		}
	}

	frpEndpointCount.Set(float64(len(r.endpoints)))
	return ctrl.Result{RequeueAfter: r.interval}, nil
}

func kubernetesClient() (ctrlclient.Client, error) {
	// 获取 kubeconfig 文件路径
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		home := os.Getenv("HOME")
		kubeconfig = filepath.Join(home, ".kube", "config")
	}
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to build kubeconfig: %v", err)
	}
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(frpv1alpha1.AddToScheme(scheme))

	cli, err := ctrlclient.New(config, ctrlclient.Options{Scheme: scheme})
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %v", err)
	}
	return cli, nil
}

func (r *FRPSServerController) getAllEndpoints(ctx context.Context) (map[string]*EndpointInfo, error) {
	logger := ctrl.Log.WithName("frps-controller")
	var list frpv1alpha1.FRPServerList
	if err := r.client.List(ctx, &list); err != nil {
		logger.Error(err, "failed to list FRPServer CRs")
		return nil, err
	}

	// Flatten each active connection into a Router record
	endpoints := make(map[string]*EndpointInfo, len(r.mappedServices))
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
			proxyName := conn.ProxyName
			info, err := kubefrp.ParseServiceName(proxyName)
			if err != nil {
				logger.Error(err, "failed to parse service name")
				continue
			}
			// 如果当前service不在需要监控的services列表中，则跳过
			if _, ok := r.mappedServices[info.Name]; !ok {
				continue
			}
			endpoints[proxyName] = &EndpointInfo{
				FrpServerAddr:   ServerAddr,
				FrpSecretKey:    sk,
				FrpServerPort:   fmt.Sprintf("%d", ServerPort),
				ServiceName:     info.Name,
				ServiceProtocol: info.Protocol,
				ServicePort:     info.Port,
				ProxyName:       proxyName,
			}
		}
	}
	return endpoints, nil
}
