package registry

import (
	"context"
	"net"
	"time"

	"github.com/coreos/etcd/clientv3"
	"github.com/sirupsen/logrus"
)

// Service 定义服务信息
type Service struct {
	Name      string
	Namespace string
	NodeGroup string
	IP        net.IP
	Port      int
	TTL       time.Duration
}

// Store 实现服务注册存储
type Store struct {
	client *clientv3.Client
	log    *logrus.Logger
}

// NewStore 创建新的服务存储
func NewStore(endpoints []string) (*Store, error) {
	client, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		return nil, err
	}

	return &Store{
		client: client,
		log:    logrus.New(),
	}, nil
}

// Register 注册服务
func (s *Store) Register(ctx context.Context, svc *Service) error {
	// TODO: 实现服务注册逻辑
	// 1. 将服务信息写入 etcd
	// 2. 设置 TTL
	// 3. 定期续约
	return nil
}

// GetService 获取服务信息
func (s *Store) GetService(ctx context.Context, name, namespace, nodeGroup string) ([]net.IP, error) {
	// TODO: 实现服务查询逻辑
	// 1. 从 etcd 查询服务信息
	// 2. 返回服务 IP 列表
	return nil, nil
}
