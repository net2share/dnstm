// Package version provides version information for dnstm.
package version

// Version and BuildTime are set at build time via ldflags.
var (
	Version   = "dev"
	BuildTime = "unknown"
)

// Set sets the version and build time.
func Set(version, buildTime string) {
	Version = version
	BuildTime = buildTime
}
