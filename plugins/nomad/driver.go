package nomad

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/api"
	"github.com/mitchellh/mapstructure"
	"github.com/mwantia/forge/pkg/errors"
	"github.com/mwantia/forge/pkg/plugins"
)

const PluginName = "nomad"

const PluginDescription = "HashiCorp Nomad workload orchestration tools"

func init() {
	plugins.Register(PluginName, PluginDescription, NewNomadDriver)
}

type NomadDriver struct {
	plugins.UnimplementedToolsPlugin
	log    hclog.Logger
	config *NomadConfig
	client *api.Client
}

type NomadConfig struct {
	Address   string     `mapstructure:"address"`
	Token     string     `mapstructure:"token"`
	Region    string     `mapstructure:"region"`
	Namespace string     `mapstructure:"namespace"`
	Timeout   int        `mapstructure:"timeout"`
	TLS       *TLSConfig `mapstructure:"tls"`
}

type TLSConfig struct {
	CAFile             string `mapstructure:"ca_file"`
	CertFile           string `mapstructure:"cert_file"`
	KeyFile            string `mapstructure:"key_file"`
	InsecureSkipVerify bool   `mapstructure:"insecure_skip_verify"`
}

func NewNomadDriver(log hclog.Logger) plugins.Driver {
	return &NomadDriver{
		log: log.Named(PluginName),
	}
}

func (d *NomadDriver) GetPluginInfo() plugins.PluginInfo {
	return plugins.PluginInfo{
		Name:        PluginName,
		Author:      "forge",
		Version:     "0.1.0",
		Description: PluginDescription,
	}
}

func (d *NomadDriver) ProbePlugin(_ context.Context) (bool, error) {
	if d.client == nil {
		return false, nil
	}
	_, err := d.client.Agent().Self()
	if err != nil {
		return false, nil
	}
	return true, nil
}

func (d *NomadDriver) GetCapabilities(_ context.Context) (*plugins.DriverCapabilities, error) {
	return &plugins.DriverCapabilities{
		Types: []string{plugins.PluginTypeTools},
		Tools: &plugins.ToolsCapabilities{
			SupportsAsyncExecution: false,
		},
	}, nil
}

func (d *NomadDriver) ConfigDriver(_ context.Context, config plugins.PluginConfig) error {
	cfg := &NomadConfig{}
	if err := mapstructure.Decode(config.ConfigMap, cfg); err != nil {
		return fmt.Errorf("failed to decode config: %w", err)
	}

	if cfg.Address == "" {
		cfg.Address = "http://127.0.0.1:4646"
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 30
	}

	d.config = cfg

	nomadCfg := api.DefaultConfig()
	nomadCfg.Address = cfg.Address
	nomadCfg.SecretID = cfg.Token
	nomadCfg.Region = cfg.Region
	nomadCfg.Namespace = cfg.Namespace

	if cfg.TLS != nil {
		tlsCfg, err := buildTLSConfig(cfg.TLS)
		if err != nil {
			return fmt.Errorf("failed to build TLS config: %w", err)
		}
		nomadCfg.HttpClient = &http.Client{
			Transport: &http.Transport{TLSClientConfig: tlsCfg},
			Timeout:   time.Duration(cfg.Timeout) * time.Second,
		}
	} else {
		nomadCfg.HttpClient = &http.Client{
			Timeout: time.Duration(cfg.Timeout) * time.Second,
		}
	}

	client, err := api.NewClient(nomadCfg)
	if err != nil {
		return fmt.Errorf("failed to create nomad client: %w", err)
	}
	d.client = client

	d.log.Info("Nomad configured", "address", cfg.Address, "region", cfg.Region)
	return nil
}

func (d *NomadDriver) OpenDriver(_ context.Context) error {
	return nil
}

func (d *NomadDriver) CloseDriver(_ context.Context) error {
	return nil
}

func (d *NomadDriver) GetProviderPlugin(_ context.Context) (plugins.ProviderPlugin, error) {
	return nil, errors.ErrPluginNotSupported
}

func (d *NomadDriver) GetMemoryPlugin(_ context.Context) (plugins.MemoryPlugin, error) {
	return nil, errors.ErrPluginNotSupported
}

func (d *NomadDriver) GetChannelPlugin(_ context.Context) (plugins.ChannelPlugin, error) {
	return nil, errors.ErrPluginNotSupported
}

func (d *NomadDriver) GetToolsPlugin(_ context.Context) (plugins.ToolsPlugin, error) {
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
