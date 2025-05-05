package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"strings"

	"github.com/imneov/servicekeel/internal/controller"
	"github.com/imneov/servicekeel/internal/dns"
)

// CLI flags
var (
	flagServices = flag.String("mapped-services", "", "comma-separated list of services to map (env: SIDECAR_MAPPED_SERVICES)")
	flagIPRange  = flag.String("ip-range", "", "CIDR notation IP range for mapping (env: SIDECAR_IP_RANGE)")
	flagDNSAddr  = flag.String("dns-addr", "", "DNS listen address (env: SIDECAR_DNS_ADDR), e.g., 127.0.0.2:53")
)

func main() {
	// parse CLI flags
	flag.Parse()

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

	// 初始化 APIServer 客户端
	apiClient, err := controller.NewFRPSServer(services, dnsServer)
	if err != nil {
		log.Fatalf("创建 APIServer 客户端失败: %v", err)
	}

	apiClient.Start()

	go func() {
		mapping := make(map[string]net.IP)
		for routersList := range routerCh {
			for _, r := range routersList {
				name := r.getName()
				ipStr := r.getIP()
				if _, want := targetServices[svc]; !want {
					continue
				}
				if _, exists := mapping[svc]; exists {
					log.Printf("服务 %s 已存在映射", svc)
					continue
				}

				ip := net.ParseIP(ipStr)
				if ip == nil {
					log.Printf("解析 IP 失败: %s", ipStr)
					continue
				}

				fqdn := fmt.Sprintf("%s.", svc)
				dnsServer.AddMapping(fqdn, ip)
			}
		}
	}()

	// TODO: 根据 Router 状态和服务列表选择合适 Router，执行 frpc 注册
	// frpcClient := frpc.NewClient(...)
	// err = frpcClient.RegisterDial(...)

	// 阻塞主进程
	select {}
}

// startDNSServer creates and starts the DNS hijacking server for sidecar.
func startDNSServer(ipRange, addr string) (*dns.Server, error) {
	server := dns.NewServer(ipRange)
	if err := server.Start(addr); err != nil {
		return nil, err
	}
	return server, nil
}
