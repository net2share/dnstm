package router

import (
	"github.com/net2share/dnstm/internal/config"
	"github.com/net2share/dnstm/internal/network"
	"github.com/net2share/dnstm/internal/transport"
)

// ServiceMode defines how a transport service should bind.
type ServiceMode string

const (
	// ServiceModeSingle binds to EXTERNAL_IP:53 (direct external access).
	ServiceModeSingle ServiceMode = "single"
	// ServiceModeMulti binds to 127.0.0.1:PORT (DNS router forwards traffic).
	ServiceModeMulti ServiceMode = "multi"
)

// ServiceGenerator handles generating service configurations with correct bindings.
type ServiceGenerator struct{}

// NewServiceGenerator creates a new service generator.
func NewServiceGenerator() *ServiceGenerator {
	return &ServiceGenerator{}
}

// GetBindOptions returns the appropriate BuildOptions for the given mode.
// For single mode: binds to EXTERNAL_IP:53
// For multi mode: binds to 127.0.0.1:cfg.Port
func (sg *ServiceGenerator) GetBindOptions(cfg *config.TunnelConfig, mode ServiceMode) (*transport.BuildOptions, error) {
	if mode == ServiceModeSingle {
		externalIP, err := network.GetExternalIP()
		if err != nil {
			return nil, err
		}
		return &transport.BuildOptions{
			BindHost: externalIP,
			BindPort: 53,
		}, nil
	}

	// Multi mode - bind to localhost on config port
	return &transport.BuildOptions{
		BindHost: "127.0.0.1",
		BindPort: cfg.Port,
	}, nil
}
