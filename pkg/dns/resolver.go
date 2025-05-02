package dns

import (
	"context"
	"net"

	"github.com/miekg/dns"
	"github.com/sirupsen/logrus"
)

// Resolver 实现基于 NodeGroup 的 DNS 解析
type Resolver struct {
	store ServiceStore
	log   *logrus.Logger
}

// ServiceStore 定义服务存储接口
type ServiceStore interface {
	GetService(ctx context.Context, name, namespace, nodeGroup string) ([]net.IP, error)
}

// NewResolver 创建新的 DNS 解析器
func NewResolver(store ServiceStore) *Resolver {
	return &Resolver{
		store: store,
		log:   logrus.New(),
	}
}

// HandleDNSRequest 处理 DNS 请求
func (r *Resolver) HandleDNSRequest(w dns.ResponseWriter, req *dns.Msg) {
	m := new(dns.Msg)
	m.SetReply(req)
	m.Authoritative = true

	// 解析域名
	domain := req.Question[0].Name
	// TODO: 实现域名解析逻辑
	// 1. 解析服务名、namespace 和 nodegroup
	// 2. 从 store 获取服务 IP
	// 3. 返回 DNS 响应

	w.WriteMsg(m)
}
