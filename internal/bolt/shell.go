package bolt

import (
	"os"
	"strings"
)

// envDumpVar names the temp file an interactive shell writes its final
// environment to, so changes (e.g. VAULT_TOKEN from `vault login`) carry back.
const envDumpVar = "__BOLT_TUI_ENVDUMP"

// envSkip is noise we never copy back into the TUI's own environment.
var envSkip = map[string]bool{
	envDumpVar: true, "_": true, "SHLVL": true, "OLDPWD": true, "PWD": true,
}

// MergeEnvDump applies NAME=VALUE lines from a shell env dump into the current
// process environment. Only new/changed vars are copied; deletions are not
// mirrored. Returns the number of variables updated. A missing file yields 0.
func MergeEnvDump(path string) int {
	if path == "" {
		return 0
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	raw := strings.TrimPrefix(string(data), "\uFEFF") // strip a possible BOM
	changed := 0
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimRight(line, "\r")
		name, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		name = strings.TrimSpace(name)
		if name == "" || envSkip[name] {
			continue
		}
		if os.Getenv(name) != value {
			_ = os.Setenv(name, value)
			changed++
		}
	}
	return changed
}
