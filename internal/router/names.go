package router

import (
	"fmt"
	"math/rand/v2"
	"regexp"
	"strings"

	"github.com/net2share/dnstm/internal/config"
)

var adjectives = []string{
	"swift", "quick", "silent", "hidden", "shadow",
	"bright", "dark", "rapid", "fast", "eager",
	"quiet", "stealth", "brave", "bold", "calm",
	"cool", "deep", "wild", "free", "pure",
	"safe", "sharp", "smart", "soft", "warm",
	"wise", "frost", "storm", "night", "dawn",
}

var nouns = []string{
	"tunnel", "stream", "channel", "bridge", "gateway",
	"path", "route", "link", "portal", "passage",
	"conduit", "relay", "proxy", "node", "point",
	"eagle", "falcon", "hawk", "raven", "wolf",
	"tiger", "lion", "bear", "fox", "owl",
	"river", "ocean", "cloud", "star", "moon",
}

var nameRegex = regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)*$`)

// GenerateName generates a random adjective-noun name.
func GenerateName() string {
	adj := adjectives[rand.IntN(len(adjectives))]
	noun := nouns[rand.IntN(len(nouns))]
	return adj + "-" + noun
}

// GenerateUniqueTag generates a unique tag that doesn't conflict with existing tunnels.
func GenerateUniqueTag(cfg *config.Config) string {
	maxAttempts := 100
	for i := 0; i < maxAttempts; i++ {
		tag := GenerateName()
		if cfg.GetTunnelByTag(tag) == nil {
			return tag
		}
	}
	// Fallback: add a random suffix
	return GenerateName() + fmt.Sprintf("-%d", rand.IntN(1000))
}

// ValidateTag validates a tag.
func ValidateTag(tag string) error {
	if tag == "" {
		return fmt.Errorf("tag cannot be empty")
	}

	if len(tag) < 3 {
		return fmt.Errorf("tag must be at least 3 characters")
	}

	if len(tag) > 63 {
		return fmt.Errorf("tag must be at most 63 characters")
	}

	if !nameRegex.MatchString(tag) {
		return fmt.Errorf("tag must start with a lowercase letter and contain only lowercase letters, numbers, and hyphens")
	}

	// Check for reserved tags
	reserved := []string{"coredns", "router", "default", "all", "none"}
	for _, r := range reserved {
		if tag == r {
			return fmt.Errorf("tag '%s' is reserved", tag)
		}
	}

	return nil
}

// NormalizeTag normalizes a tag to lowercase and replaces underscores with hyphens.
func NormalizeTag(tag string) string {
	tag = strings.ToLower(tag)
	tag = strings.ReplaceAll(tag, "_", "-")
	tag = strings.ReplaceAll(tag, " ", "-")
	return tag
}

// SuggestSimilarTags suggests similar tags if a tag is taken.
func SuggestSimilarTags(baseTag string, cfg *config.Config, count int) []string {
	suggestions := make([]string, 0, count)

	// Try adding numbers
	for i := 2; i <= count+10 && len(suggestions) < count; i++ {
		candidate := fmt.Sprintf("%s-%d", baseTag, i)
		if cfg.GetTunnelByTag(candidate) == nil {
			suggestions = append(suggestions, candidate)
		}
	}

	// Try different adjectives with the same noun
	parts := strings.Split(baseTag, "-")
	if len(parts) >= 2 {
		noun := parts[len(parts)-1]
		for _, adj := range adjectives {
			if len(suggestions) >= count {
				break
			}
			candidate := adj + "-" + noun
			if candidate != baseTag {
				if cfg.GetTunnelByTag(candidate) == nil {
					suggestions = append(suggestions, candidate)
				}
			}
		}
	}

	return suggestions
}

// GetServiceName returns the systemd service name for a tunnel.
func GetServiceName(tag string) string {
	return "dnstm-" + tag
}

// GenerateUniqueTunnelTag generates a unique tag that doesn't conflict with existing tunnels.
// This function takes a slice of tunnel configs directly.
func GenerateUniqueTunnelTag(tunnels []config.TunnelConfig) string {
	maxAttempts := 100
	existingTags := make(map[string]bool)
	for _, t := range tunnels {
		existingTags[t.Tag] = true
	}

	for i := 0; i < maxAttempts; i++ {
		tag := GenerateName()
		if !existingTags[tag] {
			return tag
		}
	}
	// Fallback: add a random suffix
	return GenerateName() + fmt.Sprintf("-%d", rand.IntN(1000))
}

// GenerateUniqueBackendTag generates a unique tag that doesn't conflict with existing backends.
func GenerateUniqueBackendTag(backends []config.BackendConfig) string {
	maxAttempts := 100
	existingTags := make(map[string]bool)
	for _, b := range backends {
		existingTags[b.Tag] = true
	}

	for i := 0; i < maxAttempts; i++ {
		tag := GenerateName()
		if !existingTags[tag] {
			return tag
		}
	}
	// Fallback: add a random suffix
	return GenerateName() + fmt.Sprintf("-%d", rand.IntN(1000))
}
