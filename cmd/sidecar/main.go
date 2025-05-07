package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"k8s.io/klog"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	zap "sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/imneov/servicekeel/internal/config"
	"github.com/imneov/servicekeel/internal/controller"
	"github.com/imneov/servicekeel/internal/dns"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// CLI flags
var (
	flagVerssion    = flag.String("version", "", "service keel version")
	flagDNSAddr     = flag.String("dns-addr", "", "DNS listen address (env: SIDECAR_DNS_ADDR), e.g., 127.0.0.2:53")
	flagMetricsAddr = flag.String("metrics-addr", "", "address for metrics and health endpoints (env: METRICS_ADDR), default :8080")
	flagIPRange     = flag.String("ip-range", "", "CIDR notation IP range for mapping (env: SIDECAR_IP_RANGE)")
)

func main() {
	// parse CLI flags
	flag.Parse()

	// initialize controller-runtime logger
	logf.SetLogger(zap.New(zap.UseDevMode(true)))

	// setup metrics and health HTTP server
	metricsAddr := *flagMetricsAddr
	if metricsAddr == "" {
		metricsAddr = os.Getenv("METRICS_ADDR")
		if metricsAddr == "" {
			metricsAddr = ":1080"
		}
	}
	go func() {
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler())
		mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
		log.Printf("Metrics and health listening on %s", metricsAddr)
		if err := http.ListenAndServe(metricsAddr, mux); err != nil {
			log.Fatalf("metrics server failed: %v", err)
		}
	}()

	// Load configuration file
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration file: %v", err)
	}

	if *flagDNSAddr != "" {
		cfg.DNS.Addr = *flagDNSAddr
	}
	if *flagIPRange != "" {
		cfg.DNS.IPRange = *flagIPRange
	}
	if *flagMetricsAddr != "" {
		cfg.Metrics.Addr = *flagMetricsAddr
	}

	klog.Infof("Configuration: \n%v", cfg.String())

	// Start DNS hijacking server
	dnsServer, err := startDNSServer(cfg.DNS.IPRange, cfg.DNS.Addr)
	if err != nil {
		log.Fatalf("Failed to start DNS hijacking server: %v", err)
	}
	defer dnsServer.Stop()
	log.Println("DNS hijacking server started, listening on", cfg.DNS.Addr)

	// Create and start controller
	ctrl, err := controller.NewController(
		cfg,
		dnsServer,
	)
	if err != nil {
		log.Fatalf("Failed to create Controller: %v", err)
	}

	err = ctrl.Start()
	if err != nil {
		log.Fatalf("Failed to start Controller: %v", err)
	}

	// Create signal channel
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Wait for signal
	sig := <-sigChan
	klog.Infof("Received signal %v, starting graceful shutdown...", sig)

	// Add cleanup code here
	// For example: close database connections, stop HTTP servers, etc.

	klog.Info("Program exited")
}

// startDNSServer creates and starts the DNS hijacking server for sidecar.
func startDNSServer(ipRange, addr string) (*dns.Server, error) {
	server, err := dns.NewServer(ipRange)
	if err != nil {
		return nil, err
	}
	if err := server.Start(addr); err != nil {
		return nil, err
	}
	return server, nil
}
