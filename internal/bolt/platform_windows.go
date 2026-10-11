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

// boltArgv wraps a bolt arg list to run on Windows, where bolt ships as a .bat
// and must be launched through cmd /c.
func boltArgv(args []string) []string {
	return append([]string{"cmd", "/c", resolveBolt()}, args...)
}

// quoteArgs renders argv as a Windows command line, matching Python's
// subprocess.list2cmdline so the preview is exactly what gets executed.
func quoteArgs(argv []string) string {
	var b strings.Builder
	for i, arg := range argv {
		if i > 0 {
			b.WriteByte(' ')
		}
		needQuote := arg == "" || strings.ContainsAny(arg, " \t")
		if needQuote {
			b.WriteByte('"')
		}
		var bs int // run of pending backslashes
		for _, c := range arg {
			switch c {
			case '\\':
				bs++
			case '"':
				b.WriteString(strings.Repeat("\\", bs*2))
				bs = 0
				b.WriteString("\\\"")
			default:
				if bs > 0 {
					b.WriteString(strings.Repeat("\\", bs))
					bs = 0
				}
				b.WriteRune(c)
			}
		}
		if bs > 0 {
			// trailing backslashes: double them if we're inside quotes
			n := bs
			if needQuote {
				n *= 2
			}
			b.WriteString(strings.Repeat("\\", n))
		}
		if needQuote {
			b.WriteByte('"')
		}
	}
	return b.String()
}
