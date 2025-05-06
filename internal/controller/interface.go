package controller

// frpsRouter adapts FRPServer CR status to router.Router interface.
// Endpoint represents a single service endpoint after expanding K8s Endpoints (or EndpointSlices).
// frpc stcp visitor
// -n apiserver-cluster-demo1-v-mac-m1-40
// --server-name 172.31.19.40:30277
// --sk abc001
// --bind-port 50277
// -s frp.thingsdao.com
type EndpointInfo struct {
	FrpServerAddr   string
	FrpServerPort   string
	FrpSecretKey    string
	ServiceName     string
	ServicePort     string
	ServiceProtocol string
	MappedIP        string             // 准备用于映射 service的 host，需要限制在范围内
	FRPClient       FRPClientInterface // 用于管理 frpc 的客户端
	ProxyName       string             // 记录 frpc 的 proxy name
}
