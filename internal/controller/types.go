package controller

type EndpointType string

const (
	EndpointTypeImported EndpointType = "imported"
	EndpointTypeExported EndpointType = "exported"
)

type EndpointInfo struct {
	// Endpoint type
	Type EndpointType
	// FRP client for managing frpc
	// - imported endpoints use frpc stcp visitor
	// - exported endpoints use frpc stcp
	FRPClient *FRPClient
	// FRP server listen address
	FrpServerListen string
	// FRP secret key
	FrpSecretKey string
	// Service name
	ServiceName string
	// Service port
	ServicePort string
	// Service protocol
	ServiceProtocol string
	// Mapped IP address to be used for service mapping, needs to be restricted within range
	MappedIP string
}
