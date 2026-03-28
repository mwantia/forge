package consul

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/go-hclog"
	"github.com/mitchellh/mapstructure"
	"github.com/mwantia/forge/pkg/errors"
	"github.com/mwantia/forge/pkg/plugins"
)

const PluginName = "consul"

const PluginDescription = "HashiCorp Consul service mesh and key-value store tools"

func init() {
	plugins.Register(PluginName, PluginDescription, NewConsulDriver)
}

type ConsulDriver struct {
	plugins.UnimplementedToolsPlugin
	log    hclog.Logger
	config *ConsulConfig
	client *api.Client
}

type ConsulConfig struct {
	Address    string     `mapstructure:"address"`
	Token      string     `mapstructure:"token"`
	Datacenter string     `mapstructure:"datacenter"`
	Namespace  string     `mapstructure:"namespace"`
	Partition  string     `mapstructure:"partition"`
	Timeout    int        `mapstructure:"timeout"`
	TLS        *TLSConfig `mapstructure:"tls"`
}

type TLSConfig struct {
	CAFile             string `mapstructure:"ca_file"`
	CertFile           string `mapstructure:"cert_file"`
	KeyFile            string `mapstructure:"key_file"`
	InsecureSkipVerify bool   `mapstructure:"insecure_skip_verify"`
}

func NewConsulDriver(log hclog.Logger) plugins.Driver {
	return &ConsulDriver{
		log: log.Named(PluginName),
	}
}

func (d *ConsulDriver) GetPluginInfo() plugins.PluginInfo {
	return plugins.PluginInfo{
		Name:        PluginName,
		Author:      "forge",
		Version:     "0.1.0",
		Description: PluginDescription,
	}
}

func (d *ConsulDriver) ProbePlugin(ctx context.Context) (bool, error) {
	if d.client == nil {
		return false, nil
	}
	_, err := d.client.Status().Leader()
	if err != nil {
		return false, nil
	}
	return true, nil
}

func (d *ConsulDriver) GetCapabilities(ctx context.Context) (*plugins.DriverCapabilities, error) {
	return &plugins.DriverCapabilities{
		Types: []string{plugins.PluginTypeTools},
		Tools: &plugins.ToolsCapabilities{
			SupportsAsyncExecution: false,
		},
	}, nil
}

func (d *ConsulDriver) ConfigDriver(ctx context.Context, config plugins.PluginConfig) error {
	cfg := &ConsulConfig{}
	if err := mapstructure.Decode(config.ConfigMap, cfg); err != nil {
		return fmt.Errorf("failed to decode config: %w", err)
	}

	if cfg.Address == "" {
		cfg.Address = "http://localhost:8500"
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 30
	}

	d.config = cfg

	consulCfg := api.DefaultConfig()
	consulCfg.Address = cfg.Address
	consulCfg.Token = cfg.Token
	consulCfg.Datacenter = cfg.Datacenter
	consulCfg.Namespace = cfg.Namespace
	consulCfg.Partition = cfg.Partition

	if cfg.TLS != nil {
		tlsCfg, err := buildTLSConfig(cfg.TLS)
		if err != nil {
			return fmt.Errorf("failed to build TLS config: %w", err)
		}
		consulCfg.HttpClient = &http.Client{
			Transport: &http.Transport{TLSClientConfig: tlsCfg},
			Timeout:   time.Duration(cfg.Timeout) * time.Second,
		}
	} else {
		consulCfg.HttpClient = &http.Client{
			Timeout: time.Duration(cfg.Timeout) * time.Second,
		}
	}

	client, err := api.NewClient(consulCfg)
	if err != nil {
		return fmt.Errorf("failed to create consul client: %w", err)
	}
	d.client = client

	d.log.Info("Consul configured", "address", cfg.Address, "datacenter", cfg.Datacenter)
	return nil
}

func (d *ConsulDriver) OpenDriver(ctx context.Context) error {
	return nil
}

func (d *ConsulDriver) CloseDriver(ctx context.Context) error {
	return nil
}

func (d *ConsulDriver) GetProviderPlugin(ctx context.Context) (plugins.ProviderPlugin, error) {
	return nil, errors.ErrPluginNotSupported
}

func (d *ConsulDriver) GetMemoryPlugin(ctx context.Context) (plugins.MemoryPlugin, error) {
	return nil, errors.ErrPluginNotSupported
}

func (d *ConsulDriver) GetChannelPlugin(ctx context.Context) (plugins.ChannelPlugin, error) {
	return nil, errors.ErrPluginNotSupported
}

func (d *ConsulDriver) GetToolsPlugin(ctx context.Context) (plugins.ToolsPlugin, error) {
	return d, nil
}

func buildTLSConfig(cfg *TLSConfig) (*tls.Config, error) {
	tlsCfg := &tls.Config{
		InsecureSkipVerify: cfg.InsecureSkipVerify, //nolint:gosec
	}

	if cfg.CAFile != "" {
		caPEM, err := os.ReadFile(cfg.CAFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA file: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(caPEM) {
			return nil, fmt.Errorf("failed to parse CA certificate")
		}
		tlsCfg.RootCAs = pool
	}

	if cfg.CertFile != "" && cfg.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load client certificate: %w", err)
		}
		tlsCfg.Certificates = []tls.Certificate{cert}
	}

	return tlsCfg, nil
}
