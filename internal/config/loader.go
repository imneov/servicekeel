package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/viper"
	"k8s.io/klog"
)

var (
	defaultConfigDir = "/etc/servicekeel"
)

// LoadConfig reads and validates the configuration files using Viper
func LoadConfig() (*Config, error) {
	// Parse configuration
	var config Config

	err := ReadConfig(&config)
	if err != nil {
		klog.Errorf("failed to read main configuration file: %v", err)
		config = Config{
			DNS: DNSConfig{
				IPRange: "127.0.66.0/24",
				Addr:    "127.0.0.2:53",
			},
			Metrics: MetricsConfig{
				Addr: ":8080",
			},
			ExportedServices: ServiceList{},
			ImportedServices: ServiceList{},
		}
	}

	// Read exported services configuration
	err = ReadExportedServicesConfig(&config)
	if err != nil {
		klog.Errorf("failed to read exported services configuration: %v, no services exported", err)
		config.ExportedServices = ServiceList{}
	}

	// Read imported services configuration
	err = ReadImportedServicesConfig(&config)
	if err != nil {
		klog.Errorf("failed to read imported services configuration: %v, no services imported", err)
		config.ImportedServices = ServiceList{}
	}

	// Validate configuration
	if err := validateServices(config.ExportedServices); err != nil {
		klog.Errorf("failed to validate exported services configuration: %v", err)
		return nil, fmt.Errorf("failed to validate exported services configuration: %v", err)
	}
	if err := validateServices(config.ImportedServices); err != nil {
		klog.Errorf("failed to validate imported services configuration: %v", err)
		return nil, fmt.Errorf("failed to validate imported services configuration: %v", err)
	}

	return &config, nil
}

func ReadConfig(config *Config) error {
	v := viper.New()

	// Set default values
	v.SetDefault("dns.addr", "127.0.0.2:53")
	v.SetDefault("dns.ipRange", "127.0.0.0/24")
	v.SetDefault("metrics.addr", ":8080")

	// Set environment variable prefix
	v.SetEnvPrefix("SIDECAR")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Set configuration file type and paths
	v.SetConfigName("config")
	v.AddConfigPath(defaultConfigDir)
	v.AddConfigPath("config")
	v.AddConfigPath(".")
	v.SetConfigType("json")
	v.SetConfigType("yaml")

	// Read configuration file
	if err := v.ReadInConfig(); err != nil {
		return fmt.Errorf("failed to read main configuration file: %v", err)
	}
	// v.WriteConfig()
	if err := v.Unmarshal(&config); err != nil {
		return fmt.Errorf("failed to parse main configuration file: %v", err)
	}
	return nil
}

func ReadExportedServicesConfig(config *Config) error {
	exportedViper := viper.New()
	exportedViper.SetConfigName("exported-services-config")
	exportedViper.AddConfigPath(defaultConfigDir)
	exportedViper.AddConfigPath("config")
	exportedViper.AddConfigPath(".")
	exportedViper.SetConfigType("yaml")
	if err := exportedViper.ReadInConfig(); err != nil {
		return fmt.Errorf("failed to read exported services configuration: %v", err)
	}
	// exportedViper.WriteConfig()
	if err := exportedViper.Unmarshal(&config.ExportedServices); err != nil {
		return fmt.Errorf("failed to parse exported services configuration: %v", err)
	}
	return nil
}

func ReadImportedServicesConfig(config *Config) error {
	importedViper := viper.New()
	importedViper.SetConfigName("imported-services-config")
	importedViper.AddConfigPath(defaultConfigDir)
	importedViper.AddConfigPath("config")
	importedViper.AddConfigPath(".")
	importedViper.SetConfigType("yaml")
	if err := importedViper.ReadInConfig(); err != nil {
		return fmt.Errorf("failed to read imported services configuration: %v", err)
	}
	// importedViper.WriteConfig()
	if err := importedViper.Unmarshal(&config.ImportedServices); err != nil {
		return fmt.Errorf("failed to parse imported services configuration: %v", err)
	}
	return nil
}

// validateServices validates the service configurations
func validateServices(services ServiceList) error {
	for _, svc := range services.Services {
		// Validate service name
		if svc.Name == "" {
			klog.Errorf("service name cannot be empty, service config: %v", svc)
			return fmt.Errorf("service name cannot be empty")
		}

		// Validate port configuration
		for _, port := range svc.Ports {
			if port.Name == "" {
				return fmt.Errorf("port name cannot be empty")
			}
			if port.Port <= 0 || port.Port > 65535 {
				return fmt.Errorf("invalid port number: %d", port.Port)
			}
			if port.TargetPort <= 0 || port.TargetPort > 65535 {
				return fmt.Errorf("invalid target port number: %d", port.TargetPort)
			}
			if port.Protocol != "TCP" && port.Protocol != "UDP" {
				return fmt.Errorf("invalid protocol type: %s", port.Protocol)
			}
		}
	}
	return nil
}

// ConvertToServiceNames converts ServiceConfig to service names
func ConvertToServiceNames(config *Config) []string {
	var serviceNames []string

	// Process exported services
	for _, svc := range config.ExportedServices.Services {
		serviceName := fmt.Sprintf("%s.%s.%s", svc.Name, svc.Namespace, svc.Cluster)
		serviceNames = append(serviceNames, serviceName)
	}

	// Process imported services
	for _, svc := range config.ImportedServices.Services {
		serviceName := fmt.Sprintf("%s.%s.%s", svc.Name, svc.Namespace, svc.Cluster)
		serviceNames = append(serviceNames, serviceName)
	}

	return serviceNames
}

// GetConfigPath returns the configuration file path
func GetConfigPath() string {
	configPath := os.Getenv("SIDECAR_CONFIG_PATH")
	if configPath == "" {
		configPath = "/etc/servicekeel/config.json"
	}
	return configPath
}

// GetDNSAddr returns the DNS server address
func GetDNSAddr() string {
	return viper.GetString("dns.addr")
}

// GetIPRange returns the IP range for DNS mapping
func GetIPRange() string {
	return viper.GetString("dns.ipRange")
}

// GetMetricsAddr returns the metrics server address
func GetMetricsAddr() string {
	return viper.GetString("metrics.addr")
}
