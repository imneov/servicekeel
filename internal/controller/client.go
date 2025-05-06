package controller

import (
	"fmt"
	"os"
	"os/exec"
)

type FRPClient struct {
	Name string
	Args []string
	Cmd  *exec.Cmd
}

func NewFRPClient(name string, args *EndpointInfo) (*FRPClient, error) {
	// frpc stcp visitor
	// -n apiserver-cluster-demo1-v-mac-m1-5
	// --server-name 1.1.1.5:3306
	// --sk abc001
	// --bind-port 3306
	// --bind-addr 127.0.66.1
	// -s frp.thingsdao.com
	if name == "" {
		name = args.ServiceName + "-" + args.MappedIP
	}
	if args.FrpServerAddr == "" {
		return nil, fmt.Errorf("FrpServerAddr is empty")
	}
	if args.FrpServerPort == "" {
		return nil, fmt.Errorf("FrpServerPort is empty")
	}
	if args.FrpSecretKey == "" {
		return nil, fmt.Errorf("FrpSecretKey is empty")
	}
	if args.MappedIP == "" {
		return nil, fmt.Errorf("MappedIP is empty")
	}
	if args.ServicePort == "" {
		return nil, fmt.Errorf("ServicePort is empty")
	}
	frpArgs := []string{
		"stcp",
		"visitor",
		"-n", name,
		"--server-name", args.ServiceName,
		"--sk", args.FrpSecretKey,
		"--bind-addr", args.MappedIP,
		"--bind-port", args.ServicePort,
		"-s", fmt.Sprintf("%s:%s", args.FrpServerAddr, args.FrpServerPort),
	}

	return &FRPClient{
		Name: name,
		Args: frpArgs,
	}, nil
}
func (c *FRPClient) Start() error {
	cmd := exec.Command("frpc", c.Args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start FRP client %s: %w", c.Name, err)
	}
	c.Cmd = cmd
	return nil
}

func (c *FRPClient) Stop() error {
	if c.Cmd != nil && c.Cmd.Process != nil {
		return c.Cmd.Process.Kill()
	}
	return nil
}
