package version

import "runtime/debug"

// Version, GitCommit, and BuildDate are set via ldflags at build time.
var (
	Version   = "dev"
	GitCommit = "unknown"
	BuildDate = "unknown"
)

// Effective returns the build version. When the binary was built without
// ldflags (for example `go install github.com/gburgyan/aat/cmd/aat@v0.1.0`),
// it falls back to the module version recorded by the Go toolchain.
func Effective() string {
	if Version != "" && Version != "dev" {
		return Version
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		if v := info.Main.Version; v != "" && v != "(devel)" {
			return v
		}
	}
	return Version
}
