//go:build !windows

package ui

import (
	"bufio"
	"io"
	"os"
	"os/signal"
)

// waitForExitKey waits until the user presses Enter or Ctrl+C at the pause
// prompt, then returns so bolt-navi can resume its TUI.
//
// On non-Windows, Ctrl+C continues to raise SIGINT. We intercept it via
// signal.Notify so the process does not terminate at the prompt.
func waitForExitKey(r io.Reader) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	defer signal.Stop(sigCh)

	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = bufio.NewReader(r).ReadString('\n')
	}()
	select {
	case <-done:
	case <-sigCh:
	}
}
