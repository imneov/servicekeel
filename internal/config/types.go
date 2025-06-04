package config

import (
	"bytes"
	"fmt"

	"gopkg.in/yaml.v3"
)

// ServiceConfig represents the configuration for a service
type ServiceConfig struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Cluster   string `json:"cluster"`
	Ports     []Port `json:"ports"`
}

// Port represents a service port configuration
type Port struct {
	Name       string `json:"name"`
	Port       int    `json:"port"`
	TargetPort int    `json:"targetPort"`
	Protocol   string `json:"protocol"`
}

// ServiceList represents the list of services in the configuration
type ServiceList struct {
	Services []ServiceConfig `json:"services"`
}

// Config represents the complete configuration
type Config struct {
	DNS              DNSConfig     `json:"dns"`
	Metrics          MetricsConfig `json:"metrics"`
	ExportedServices ServiceList   `json:"exported"`
	ImportedServices ServiceList   `json:"imported"`
}

func (c *Config) String() string {
	var buf bytes.Buffer
	err := yaml.NewEncoder(&buf).Encode(c)
	if err != nil {
		return fmt.Sprintf("failed to marshal config: %v", err)
	}
	return buf.String()
}

type DNSConfig struct {
	IPRange       string   `json:"ipRange"`
	Addr          string   `json:"addr"`
	SearchDomains []string `json:"searchDomains"`
}

type MetricsConfig struct {
	Addr string `json:"addr"`
}
