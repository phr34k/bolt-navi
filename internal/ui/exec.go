package ui

import (
	"fmt"
	"io"
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/phr34k/bolt-navi/internal/bolt"
)

// pausingCmd adapts an *exec.Cmd to Bubble Tea's ExecCommand interface and
// reproduces the Python run_command flow: print the command, run it with live
// output on the restored main screen, then wait for Enter so the output stays
// visible before the TUI re-enters its alt-screen. Without this pause Bubble
// Tea repaints over bolt's output the instant it exits.
type pausingCmd struct {
	cmd     *exec.Cmd
	preview string
	in      io.Reader
	out     io.Writer
}

func (p *pausingCmd) SetStdin(r io.Reader)  { p.in = r; p.cmd.Stdin = r }
func (p *pausingCmd) SetStdout(w io.Writer) { p.out = w; p.cmd.Stdout = w }
func (p *pausingCmd) SetStderr(w io.Writer) { p.cmd.Stderr = w }

func (p *pausingCmd) Run() error {
	if p.out != nil {
		fmt.Fprintf(p.out, "$ %s\n\n", p.preview)
	}
	err := p.cmd.Run()
	if p.out != nil {
		fmt.Fprintf(p.out, "\n[bolt exited %d -- press Enter or Ctrl+C to return] ", exitCode(err))
	}
	if p.in != nil {
		waitForExitKey(p.in)
	}
	if p.out != nil {
		fmt.Fprintf(p.out, "\r\033[2K")
	}
	return err
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	if ee, ok := err.(*exec.ExitError); ok {
		return ee.ExitCode()
	}
	return -1
}

// runExec builds the tea.Cmd that runs bolt with the pause-to-return behavior.
func runExec(root, name string, argv []string, preview string) tea.Cmd {
	pc := &pausingCmd{cmd: bolt.BoltCmd(root, argv), preview: preview}
	return tea.Exec(pc, func(err error) tea.Msg {
		return runDoneMsg{name: name, code: exitCode(err)}
	})
}
