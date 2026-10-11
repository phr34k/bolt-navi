//go:build !windows

package bolt

import (
	"os/exec"
	"strings"
	"sync"
)

var boltOnce sync.Once
var boltPath = "bolt"

func resolveBolt() string {
	boltOnce.Do(func() {
		if p, err := exec.LookPath("bolt"); err == nil {
			boltPath = p
		}
	})
	return boltPath
}

// boltArgv runs bolt directly on POSIX systems.
func boltArgv(args []string) []string {
	return append([]string{resolveBolt()}, args...)
}

// quoteArgs renders argv with POSIX shell quoting (equivalent of shlex.quote
// on each element), so the preview is safe to copy-paste into a shell.
func quoteArgs(argv []string) string {
	parts := make([]string, len(argv))
	for i, a := range argv {
		parts[i] = shellQuote(a)
	}
	return strings.Join(parts, " ")
}

func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	if !strings.ContainsAny(s, " \t\n\"'\\$`&|;<>()*?!#~[]{}") {
		return s
	}
	// Wrap in single quotes, escaping any embedded single quote as '\''.
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
