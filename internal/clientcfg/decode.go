package clientcfg

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
)

// Decode parses a dnst:// URL string into a ClientConfig.
func Decode(url string) (*ClientConfig, error) {
	if !strings.HasPrefix(url, urlPrefix) {
		return nil, fmt.Errorf("invalid URL: missing %s prefix", urlPrefix)
	}

	encoded := strings.TrimPrefix(url, urlPrefix)
	if encoded == "" {
		return nil, fmt.Errorf("invalid URL: empty payload")
	}

	data, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64: %w", err)
	}

	var cfg ClientConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	if cfg.Version < 1 {
		return nil, fmt.Errorf("unsupported config version: %d", cfg.Version)
	}

	return &cfg, nil
}
