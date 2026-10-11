package ui

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const donateURL = "https://github.com/sponsors/phr34k"

const donateEvery = 15 // show dialog every Nth startup

const (
	footerHints = " ↑↓ move · Enter configure & run · Ctrl+T shell · Esc clear / quit "
	donateLabel = "Ctrl+D donate"
)

// Embedded signing key. Prevents casual file editing from silently resetting
// the counter — the HMAC won't match and the dialog fires immediately.
var counterKey = []byte{
	0xe3, 0x1a, 0x7f, 0x54, 0xc2, 0x98, 0x0b, 0xd6,
	0x45, 0xfa, 0x37, 0x2c, 0x81, 0x69, 0xbe, 0x4e,
	0xd0, 0x73, 0xa5, 0x1c, 0x96, 0x2f, 0x58, 0x84,
	0x3b, 0xe7, 0xc1, 0x0d, 0x47, 0x6a, 0xf9, 0x22,
}

func signCounter(n int) string {
	mac := hmac.New(sha256.New, counterKey)
	fmt.Fprintf(mac, "%d", n)
	return hex.EncodeToString(mac.Sum(nil))
}

func verifyCounter(n int, sig string) bool {
	got, err := hex.DecodeString(strings.TrimSpace(sig))
	if err != nil {
		return false
	}
	want, _ := hex.DecodeString(signCounter(n))
	return hmac.Equal(got, want)
}

func donateHyperlink(label string) string {
	return "\x1b]8;;" + donateURL + "\x1b\\" + label + "\x1b]8;;\x1b\\"
}

func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}

func startupCountFile() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "bolt-navi", "startups")
}

// incrementStartupCount bumps the persistent counter and returns the new value.
// The file stores "count:hmac" so editing the number alone breaks the signature,
// causing the dialog to show on the next launch.
func incrementStartupCount() int {
	path := startupCountFile()
	if path == "" {
		return 0
	}

	var n int
	if data, err := os.ReadFile(path); err == nil {
		parts := strings.SplitN(strings.TrimSpace(string(data)), ":", 2)
		if len(parts) == 2 {
			fmt.Sscanf(parts[0], "%d", &n)
			if !verifyCounter(n, parts[1]) {
				// Tampered: fire the dialog this run.
				n = donateEvery - 1
			}
		}
	}

	n++
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return n
	}
	os.WriteFile(path, []byte(fmt.Sprintf("%d:%s", n, signCounter(n))), 0o644) //nolint:errcheck
	return n
}

func shouldShowDonateDialog(count int) bool {
	return count > 0 && count%donateEvery == 0
}
