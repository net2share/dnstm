package updater

import (
	"github.com/net2share/dnstm/internal/binary"
	"github.com/net2share/dnstm/internal/config"
	"github.com/net2share/dnstm/internal/proxy"
	"github.com/net2share/dnstm/internal/router"
	"github.com/net2share/dnstm/internal/service"
)

// ServiceInfo contains information about a running service.
type ServiceInfo struct {
	Name   string
	Binary binary.BinaryType
}

// GetActiveServicesForBinary returns active services that use the specified binary.
func GetActiveServicesForBinary(binType binary.BinaryType) []string {
	var services []string

	switch binType {
	case binary.BinaryMicrosocks:
		if proxy.IsMicrosocksRunning() {
			services = append(services, proxy.MicrosocksServiceName)
		}

	case binary.BinarySlipstreamServer, binary.BinarySSServer, binary.BinaryDNSTTServer:
		// Check tunnel services
		cfg, err := config.Load()
		if err != nil || cfg == nil {
			return services
		}

		for _, tunnelCfg := range cfg.Tunnels {
			tunnel := router.NewTunnel(&tunnelCfg)
			if !tunnel.IsActive() {
				continue
			}

			// Check if this tunnel uses the binary
			if tunnelUsesBinary(&tunnelCfg, binType) {
				services = append(services, tunnel.ServiceName)
			}
		}
	}

	return services
}

// tunnelUsesBinary checks if a tunnel configuration uses the specified binary.
func tunnelUsesBinary(tunnelCfg *config.TunnelConfig, binType binary.BinaryType) bool {
	switch binType {
	case binary.BinarySlipstreamServer:
		return tunnelCfg.Transport == config.TransportSlipstream

	case binary.BinarySSServer:
		// ssserver is used when backend is shadowsocks
		cfg, err := config.Load()
		if err != nil || cfg == nil {
			return false
		}
		backend := cfg.GetBackendByTag(tunnelCfg.Backend)
		if backend == nil {
			return false
		}
		return backend.Type == config.BackendShadowsocks

	case binary.BinaryDNSTTServer:
		return tunnelCfg.Transport == config.TransportDNSTT
	}

	return false
}

// StopServices stops the specified services and returns the list of stopped services.
func StopServices(serviceNames []string) []string {
	var stopped []string

	for _, name := range serviceNames {
		if err := service.StopService(name); err == nil {
			stopped = append(stopped, name)
		}
	}

	return stopped
}

// StartServices starts the specified services.
func StartServices(serviceNames []string) error {
	for _, name := range serviceNames {
		if err := service.StartService(name); err != nil {
			return err
		}
	}
	return nil
}

// GetAllActiveServices returns all active services that might be affected by updates.
func GetAllActiveServices() map[binary.BinaryType][]string {
	result := make(map[binary.BinaryType][]string)

	binaries := []binary.BinaryType{
		binary.BinarySlipstreamServer,
		binary.BinarySSServer,
		binary.BinaryMicrosocks,
		// Note: dnstt-server is skipped for updates, but we still track its services
		binary.BinaryDNSTTServer,
	}

	for _, binType := range binaries {
		services := GetActiveServicesForBinary(binType)
		if len(services) > 0 {
			result[binType] = services
		}
	}

	return result
}
