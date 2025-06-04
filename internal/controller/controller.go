package controller

import (
	"fmt"
	"sync"

	"github.com/imneov/servicekeel/internal/config"
	"github.com/imneov/servicekeel/internal/dns"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	frpEndpointCount = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "servicekeel_frp_endpoint_count",
		Help: "Number of active FRP endpoints currently watched",
	})
)

func init() {
	prometheus.MustRegister(frpEndpointCount)
}

// Controller manages FRP client connections and DNS mappings
type Controller struct {
	config            *config.Config
	dnsServer         *dns.Server
	importedEndpoints map[string]*EndpointInfo
	exportedEndpoints map[string]*EndpointInfo
	lock              sync.RWMutex
}

// NewController creates a new controller and initializes all FRP connections
func NewController(cfg *config.Config, dnsServer *dns.Server) (*Controller, error) {
	r := &Controller{
		config:            cfg,
		dnsServer:         dnsServer,
		importedEndpoints: make(map[string]*EndpointInfo),
		exportedEndpoints: make(map[string]*EndpointInfo),
	}

	return r, nil
}

// Start reconciliation
func (r *Controller) Start() error {
	// Since the configuration is static, no periodic reconciliation is needed

	exportedServices := r.config.ExportedServices
	importedServices := r.config.ImportedServices

	// Handle exported services
	for _, svc := range exportedServices.Services {
		for _, port := range svc.Ports {
			// Create service name
			serviceName := ServiceName(svc, port)
			proxyName := EndpointName(svc, port)

			// Create and start FRP client
			endpoint := &EndpointInfo{
				Type:            EndpointTypeExported,
				ServiceName:     serviceName,
				ServicePort:     fmt.Sprintf("%d", port.Port),
				ServiceProtocol: port.Protocol,
				FrpServerListen: "/tmp/frp.sock",
				FrpSecretKey:    "servicekeel-secret-key",
			}

			frpClient, err := NewFRPClient(proxyName, endpoint)
			if err != nil {
				return fmt.Errorf("failed to create FRP client %s: %v", proxyName, err)
			}

			if err := frpClient.Start(); err != nil {
				return fmt.Errorf("failed to start FRP client %s: %v", proxyName, err)
			}

			endpoint.FRPClient = frpClient
			r.exportedEndpoints[proxyName] = endpoint
			frpEndpointCount.Inc()
		}
	}

	// Handle imported services
	for _, svc := range importedServices.Services {
		for _, port := range svc.Ports {
			// Create service name
			serviceName := ServiceName(svc, port)
			proxyName := EndpointName(svc, port)

			// Add DNS mapping
			mappedIP, err := r.dnsServer.AddMapping(serviceName)
			if err != nil {
				r.dnsServer.RemoveMapping(serviceName)
				return fmt.Errorf("failed to add DNS mapping %s: %v", serviceName, err)
			}

			// Create and start FRP client
			endpoint := &EndpointInfo{
				Type:            EndpointTypeImported,
				ServiceName:     serviceName,
				ServicePort:     fmt.Sprintf("%d", port.Port),
				ServiceProtocol: port.Protocol,
				MappedIP:        mappedIP.String(),
				FrpServerListen: "/tmp/frp.sock",
				FrpSecretKey:    "servicekeel-secret-key",
			}

			frpClient, err := NewFRPClient(proxyName, endpoint)
			if err != nil {
				r.dnsServer.RemoveMapping(serviceName)
				return fmt.Errorf("failed to create FRP client %s: %v", proxyName, err)
			}

			if err := frpClient.Start(); err != nil {
				r.dnsServer.RemoveMapping(serviceName)
				return fmt.Errorf("failed to start FRP client %s: %v", proxyName, err)
			}

			endpoint.FRPClient = frpClient
			r.importedEndpoints[proxyName] = endpoint
			frpEndpointCount.Inc()
		}
	}

	return nil
}

// GetEndpoint returns the endpoint info for a given proxy name
func (r *Controller) GetImportedEndpoint(proxyName string) *EndpointInfo {
	r.lock.RLock()
	defer r.lock.RUnlock()
	return r.importedEndpoints[proxyName]
}

func (r *Controller) GetExportedEndpoint(proxyName string) *EndpointInfo {
	r.lock.RLock()
	defer r.lock.RUnlock()
	return r.exportedEndpoints[proxyName]
}

// GetAllEndpoints returns all endpoint infos
func (r *Controller) GetAllImportedEndpoints() map[string]*EndpointInfo {
	r.lock.RLock()
	defer r.lock.RUnlock()
	return r.importedEndpoints
}

func (r *Controller) GetAllExportedEndpoints() map[string]*EndpointInfo {
	r.lock.RLock()
	defer r.lock.RUnlock()
	return r.exportedEndpoints
}

func ServiceName(svc config.ServiceConfig, port config.Port) string {
	return fmt.Sprintf("%s.%s.svc.%s", svc.Name, svc.Namespace, svc.Cluster)
}

func EndpointName(svc config.ServiceConfig, port config.Port) string {
	return fmt.Sprintf("%s:%s", ServiceName(svc, port), port.Name)
}
