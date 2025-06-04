package controller

import (
	"fmt"
	"os"
	"os/exec"

	"k8s.io/klog"
)

type FRPClient struct {
	Name string
	Args []string
	Cmd  *exec.Cmd
}

// NewFRPClient creates a new FRP client
// name: frpc proxy name
// args: endpoint info
func NewFRPClient(name string, args *EndpointInfo) (*FRPClient, error) {
	client := &FRPClient{
		Name: name,
	}
	if name == "" {
		name = args.ServiceName + "-" + args.MappedIP
	}
	if args.FrpServerListen == "" {
		return nil, fmt.Errorf("FrpServerListen is empty")
	}
	if args.FrpSecretKey == "" {
		return nil, fmt.Errorf("FrpSecretKey is empty")
	}
	if args.ServicePort == "" {
		return nil, fmt.Errorf("ServicePort is empty")
	}
	switch args.Type {
	case EndpointTypeImported:
		// e.g.
		// frpc stcp visitor
		// -n 172.31.19.5:80/123
		// --server-name 172.31.19.5:80/123
		// --sk servicekeel-secret-key
		// --bind-addr 127.0.0.1
		// --bind-port 48001
		// --server-listen /tmp/frp.sock
		if args.MappedIP == "" {
			return nil, fmt.Errorf("MappedIP is empty")
		}
		client.Args = []string{
			"stcp",
			"visitor",
			"-n", name,
			"--server-name", name,
			"--sk", args.FrpSecretKey,
			"--bind-addr", args.MappedIP,
			"--bind-port", args.ServicePort,
			"--server-listen", args.FrpServerListen,
		}
	case EndpointTypeExported:
		// e.g.
		// frpc stcp
		// --sk servicekeel-secret-key
		// -n 172.31.19.5:80/123
		// --local-ip 172.31.19.5 --local-port 80
		// --server-listen /tmp/frp.sock
		client.Args = []string{
			"stcp",
			"server",
			"-n", name,
			"--sk", args.FrpSecretKey,
			// "--local-ip", args.MappedIP,
			"--local-port", args.ServicePort,
			"--server-listen", args.FrpServerListen,
		}
	case EndpointTypeRelay:
		// e.g.
		// frpc stcp relay
		// -n source-server/target-server
		// --source-server source-frps:port
		// --target-server target-frps:port
		// --sk servicekeel-secret-key
		// --server-listen /tmp/frp.sock
		if args.SourceServer == "" {
			return nil, fmt.Errorf("SourceServer is empty")
		}
		if args.TargetServer == "" {
			return nil, fmt.Errorf("TargetServer is empty")
		}
		client.Args = []string{
			"stcp",
			"relay",
			"-n", name,
			"--source-server", args.SourceServer,
			"--target-server", args.TargetServer,
			"--sk", args.FrpSecretKey,
			"--server-listen", args.FrpServerListen,
		}
	default:
		return nil, fmt.Errorf("invalid endpoint type: %s", args.Type)
	}

	return client, nil
}

func (c *FRPClient) Start() error {
	cmd := exec.Command("frpc", c.Args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	klog.Infof("Starting FRP client %s: %v", c.Name, c.Args)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start FRP client %s: %w", c.Name, err)
	}
	c.Cmd = cmd
	return nil
}

func (c *FRPClient) Stop() error {
	klog.Infof("Stopping FRP client %s: %v", c.Name, c.Args)
	if c.Cmd != nil && c.Cmd.Process != nil {
		return c.Cmd.Process.Kill()
	}
	return nil
}
