package router

import (
	"context"
	"net/url"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// Router 实现请求路由
type Router struct {
	store ServiceStore
	log   *logrus.Logger
}

// ServiceStore 定义服务存储接口
type ServiceStore interface {
	GetService(ctx context.Context, name, namespace, nodeGroup string) ([]string, error)
}

// NewRouter 创建新的路由
func NewRouter(store ServiceStore) *Router {
	return &Router{
		store: store,
		log:   logrus.New(),
	}
}

// HandleRequest 处理 HTTP 请求
func (r *Router) HandleRequest(c *gin.Context) {
	// 解析请求域名
	host := c.Request.Host
	// TODO: 实现路由逻辑
	// 1. 解析服务名、namespace 和 nodegroup
	// 2. 从 store 获取服务地址
	// 3. 转发请求到目标服务
}

// ProxyRequest 转发请求到目标服务
func (r *Router) ProxyRequest(c *gin.Context, target *url.URL) {
	// TODO: 实现请求转发逻辑
	// 1. 修改请求头
	// 2. 转发请求
	// 3. 处理响应
}
