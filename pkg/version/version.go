// Package version provides build version information set via ldflags.
package version

// version is set at build time via ldflags:
//
//	-X github.com/fenio/pv-mounter/pkg/version.version=<tag>
var version string

// Version returns the build version, or "dev" if not set.
func Version() string {
	if version == "" {
		return "dev"
	}
	return version
}
