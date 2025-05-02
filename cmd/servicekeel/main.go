package main

import (
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/imneov/servicekeel/pkg/dns"
	"github.com/imneov/servicekeel/pkg/registry"
	"github.com/imneov/servicekeel/pkg/router"
	"github.com/sirupsen/logrus"
)

func main() {
	// 解析命令行参数
	etcdEndpoints := flag.String("etcd-endpoints", "http://localhost:2379", "etcd endpoints")
	dnsPort := flag.String("dns-port", "53", "DNS server port")
	httpPort := flag.String("http-port", "80", "HTTP server port")
	flag.Parse()

	// 初始化日志
	log := logrus.New()
	log.SetFormatter(&logrus.JSONFormatter{})

	// 初始化服务存储
	store, err := registry.NewStore([]string{*etcdEndpoints})
	if err != nil {
		log.Fatalf("Failed to create store: %v", err)
	}

	// 初始化 DNS 服务器
	dnsServer := dns.NewResolver(store)
	go func() {
		server := &dns.Server{
			Addr:    ":" + *dnsPort,
			Net:     "udp",
			Handler: dnsServer,
		}
		if err := server.ListenAndServe(); err != nil {
			log.Fatalf("Failed to start DNS server: %v", err)
		}
	}()

	// 初始化 HTTP 服务器
	router := router.NewRouter(store)
	// TODO: 设置路由和处理函数

	// 等待信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	// 清理资源
	// TODO: 实现资源清理逻辑
}
