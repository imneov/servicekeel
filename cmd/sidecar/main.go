package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"strings"

	logf "sigs.k8s.io/controller-runtime/pkg/log"
	zap "sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/imneov/servicekeel/internal/controller"
	"github.com/imneov/servicekeel/internal/dns"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// CLI flags
var (
	flagServices    = flag.String("mapped-services", "", "comma-separated list of services to map (env: SIDECAR_MAPPED_SERVICES)")
	flagIPRange     = flag.String("ip-range", "", "CIDR notation IP range for mapping (env: SIDECAR_IP_RANGE)")
	flagDNSAddr     = flag.String("dns-addr", "", "DNS listen address (env: SIDECAR_DNS_ADDR), e.g., 127.0.0.2:53")
	flagMetricsAddr = flag.String("metrics-addr", "", "address for metrics and health endpoints (env: METRICS_ADDR), default :8080")
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
			metricsAddr = ":8080"
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

	// determine mapped services
	servicesEnv := *flagServices
	if servicesEnv == "" {
		servicesEnv = os.Getenv("SIDECAR_MAPPED_SERVICES")
	}
	// determine IP range
	ipRange := *flagIPRange
	if ipRange == "" {
		ipRange = os.Getenv("SIDECAR_IP_RANGE")
	}
	if servicesEnv == "" || ipRange == "" {
		log.Fatal("必须通过环境变量 SIDECAR_MAPPED_SERVICES 和 SIDECAR_IP_RANGE 或者 --mapped-services 和 --ip-range 参数设置映射服务和 IP 段")
	}
	services := strings.Split(servicesEnv, ",")
	log.Printf("映射服务: %v", services)
	log.Printf("IP 范围: %s", ipRange)

	if len(services) > 100 {
		log.Fatal("映射服务数量不能超过 100 个")
	}
	if ipRange == "" {
		log.Fatal("IP 范围不能为空")
	}

	// determine DNS listen address
	dnsAddr := *flagDNSAddr
	if dnsAddr == "" {
		dnsAddr = os.Getenv("SIDECAR_DNS_ADDR")
	}
	if dnsAddr == "" {
		dnsAddr = "127.0.0.2:53"
	}
	log.Printf("DNS 监听地址: %s", dnsAddr)

	// 启动 DNS 劫持服务器
	dnsServer, err := startDNSServer(ipRange, dnsAddr)
	if err != nil {
		log.Fatalf("启动 DNS 劫持服务器失败: %v", err)
	}
	defer dnsServer.Stop()
	log.Println("DNS 劫持服务器已启动，监听", dnsAddr)

	// 构建目标服务集合
	targetServices := make(map[string]struct{})
	for _, svc := range services {
		targetServices[svc] = struct{}{}
	}

	// 初始化 controller
	controller, err := controller.NewFRPSServer(services, dnsServer)
	if err != nil {
		log.Fatalf("创建 APIServer 客户端失败: %v", err)
	}

	controller.Start()

	// 阻塞主进程
	select {}
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
