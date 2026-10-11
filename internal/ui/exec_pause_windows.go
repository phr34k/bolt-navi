//go:build windows

package ui

import (
	"io"
	"os"

	"golang.org/x/sys/windows"
)

// Relevant Windows console-mode flags.
const (
	enableProcessedInput = uint32(0x0001) // Ctrl+C → SIGINT; backspace/tab processing
	enableLineInput      = uint32(0x0002) // buffer input until Enter
	enableEchoInput      = uint32(0x0004) // echo typed characters
)

// waitForExitKey waits until the user presses Enter or Ctrl+C at the pause
// prompt, then returns so bolt-navi can resume its TUI.
//
// It temporarily sets the console to raw mode (no line-buffering, no signal
// processing, no echo). With ENABLE_PROCESSED_INPUT cleared, Ctrl+C delivers
// byte 0x03 into the input stream rather than raising SIGINT, eliminating any
// race with BubbleTea's own signal handler.
func waitForExitKey(r io.Reader) {
	f, ok := r.(*os.File)
	if !ok {
		waitForNewline(r)
		return
	}

	handle := windows.Handle(f.Fd())
	var oldMode uint32
	if err := windows.GetConsoleMode(handle, &oldMode); err != nil {
		waitForNewline(r)
		return
	}

	rawMode := oldMode &^ (enableProcessedInput | enableLineInput | enableEchoInput)
	if err := windows.SetConsoleMode(handle, rawMode); err != nil {
		waitForNewline(r)
		return
	}
	defer windows.SetConsoleMode(handle, oldMode) //nolint:errcheck

	buf := make([]byte, 1)
	for {
		n, err := f.Read(buf)
		if err != nil || n == 0 {
			return
		}
		if buf[0] == '\r' || buf[0] == '\n' || buf[0] == 3 { // 3 = Ctrl+C
			return
		}
	}
}

// waitForNewline is the fallback when r is not a real console handle.
func waitForNewline(r io.Reader) {
	buf := make([]byte, 1)
	for {
		n, err := r.Read(buf)
		if err != nil || n == 0 {
			return
		}
		if buf[0] == '\r' || buf[0] == '\n' {
			return
		}
	}
}
