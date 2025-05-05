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
	ServiceIP       string // 准备用于生成service的 host，需要限制在范围内
}
