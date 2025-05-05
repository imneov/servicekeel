package controller

type FRPClientPool map[string]*FRPClient

type FRPClient struct {
}

func (c *FRPClient) Start() {
}

func (c *FRPClient) Stop() {
}

func NewFRPClientPool() *FRPClientPool {

	return &FRPClientPool{}
}
