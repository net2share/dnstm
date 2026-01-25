package tunnel

import (
	"fmt"
	"sync"
)

var (
	registry = make(map[ProviderType]Provider)
	mu       sync.RWMutex
)

// Register adds a provider to the registry.
func Register(p Provider) {
	mu.Lock()
	defer mu.Unlock()
	registry[p.Name()] = p
}

// Get returns a provider by type.
func Get(pt ProviderType) (Provider, error) {
	mu.RLock()
	defer mu.RUnlock()
	p, ok := registry[pt]
	if !ok {
		return nil, fmt.Errorf("unknown provider: %s", pt)
	}
	return p, nil
}

// GetAll returns all registered providers.
func GetAll() []Provider {
	mu.RLock()
	defer mu.RUnlock()
	providers := make([]Provider, 0, len(registry))
	for _, p := range registry {
		providers = append(providers, p)
	}
	return providers
}

// GetInstalled returns all installed providers.
func GetInstalled() []Provider {
	mu.RLock()
	defer mu.RUnlock()
	var providers []Provider
	for _, p := range registry {
		if p.IsInstalled() {
			providers = append(providers, p)
		}
	}
	return providers
}

// Types returns all available provider types.
func Types() []ProviderType {
	return []ProviderType{ProviderSlipstream, ProviderDNSTT}
}

// ParseProviderType parses a string into a ProviderType.
func ParseProviderType(s string) (ProviderType, error) {
	switch s {
	case "dnstt":
		return ProviderDNSTT, nil
	case "slipstream":
		return ProviderSlipstream, nil
	default:
		return "", fmt.Errorf("unknown provider: %s (valid: dnstt, slipstream)", s)
	}
}
