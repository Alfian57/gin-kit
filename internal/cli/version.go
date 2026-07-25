package cli

import (
	"runtime/debug"
	"strings"
)

// Version is set at release time by GoReleaser. Local development builds use
// Go module build metadata when available.
var Version = "dev"

func effectiveVersion() string {
	moduleVersion := ""
	if info, ok := debug.ReadBuildInfo(); ok {
		moduleVersion = info.Main.Version
	}

	return resolveVersion(Version, moduleVersion)
}

func resolveVersion(injected, module string) string {
	if injected != "" && injected != "dev" {
		return strings.TrimPrefix(injected, "v")
	}
	if module != "" && module != "(devel)" {
		return strings.TrimPrefix(module, "v")
	}
	return "dev"
}
