package plugins

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/hashicorp/go-hclog"
	goplugin "github.com/hashicorp/go-plugin"
)

// ClientConfig configures how to connect to a plugin.
type ClientConfig struct {
	// Path to the plugin executable.
	PluginPath string

	// Arguments to pass to the plugin executable.
	Args []string

	// Logger for plugin communication.
	Logger hclog.Logger

	// Additional handshake configuration (optional).
	Handshake *goplugin.HandshakeConfig
}

// Client manages the lifecycle of a plugin connection.
type Client struct {
	client *goplugin.Client
	driver Driver
}

// NewClient creates a new plugin client.
func NewClient(cfg ClientConfig) *Client {
	handshake := Handshake
	if cfg.Handshake != nil {
		handshake = *cfg.Handshake
	}

	cmd := exec.Command(cfg.PluginPath, cfg.Args...)

	client := goplugin.NewClient(&goplugin.ClientConfig{
		HandshakeConfig:  handshake,
		Plugins:          map[string]goplugin.Plugin{"driver": &DriverPlugin{}},
		Cmd:              cmd,
		AllowedProtocols: []goplugin.Protocol{goplugin.ProtocolGRPC},
		Logger:           hclog.Default().Named("plugin"),
	})

	return &Client{client: client}
}

// NewClientFromCmd creates a new plugin client from an existing command.
func NewClientFromCmd(cmd *exec.Cmd) *Client {
	client := goplugin.NewClient(&goplugin.ClientConfig{
		HandshakeConfig:  Handshake,
		Plugins:          map[string]goplugin.Plugin{"driver": &DriverPlugin{}},
		Cmd:              cmd,
		AllowedProtocols: []goplugin.Protocol{goplugin.ProtocolGRPC},
		Logger:           hclog.Default().Named("plugin"),
	})

	return &Client{client: client}
}

// Start launches the plugin process and returns the Driver interface.
func (c *Client) Start(ctx context.Context) (Driver, error) {
	grpcClient, err := c.client.Client()
	if err != nil {
		// Check if the plugin process exited
		if exitErr := c.client.Exited(); exitErr {
			return nil, fmt.Errorf("plugin process exited unexpectedly: %w", err)
		}
		return nil, fmt.Errorf("failed to get gRPC client: %w", err)
	}

	raw, err := grpcClient.Dispense("driver")
	if err != nil {
		// Check if the plugin process exited
		if exitErr := c.client.Exited(); exitErr {
			return nil, fmt.Errorf("plugin process exited during dispense: %w", err)
		}
		return nil, fmt.Errorf("failed to dispense driver plugin: %w", err)
	}

	c.driver = raw.(Driver)
	return c.driver, nil
}

// Stop gracefully shuts down the plugin.
func (c *Client) Stop() {
	c.client.Kill()
}

// Driver returns the current driver interface, or nil if not started.
func (c *Client) Driver() Driver {
	return c.driver
}
