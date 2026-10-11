//go:build !windows

package bolt

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// ShellCmd builds an interactive shell to drop into from the TUI. For
// bash/sh it installs an EXIT trap (via a generated rcfile) that dumps the
// environment to dumpPath so MergeEnvDump can carry changes back. dumpPath is
// "" for shells we can't capture, signalling "no merge".
func ShellCmd() (cmd *exec.Cmd, dumpPath string, cleanup func(), err error) {
	dump, err := os.CreateTemp("", "bolt_tui_env_*.txt")
	if err != nil {
		return nil, "", func() {}, err
	}
	dumpPath = dump.Name()
	dump.Close()

	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/bash"
	}
	env := append(CleanEnviron(), envDumpVar+"="+dumpPath)

	base := filepath.Base(shell)
	if base == "bash" || base == "sh" {
		rc, rerr := os.CreateTemp("", "bolt_tui_*.sh")
		if rerr != nil {
			os.Remove(dumpPath)
			return nil, "", func() {}, rerr
		}
		fmt.Fprintf(rc,
			"[ -f ~/.bashrc ] && . ~/.bashrc 2>/dev/null\n"+
				"trap 'env > \"$%s\"' EXIT\n", envDumpVar)
		rcName := rc.Name()
		rc.Close()

		cmd = exec.Command(shell, "--rcfile", rcName, "-i")
		cmd.Env = env
		cleanup = func() { os.Remove(dumpPath); os.Remove(rcName) }
		return cmd, dumpPath, cleanup, nil
	}

	cmd = exec.Command(shell)
	cmd.Env = env
	cleanup = func() { os.Remove(dumpPath) }
	return cmd, "", cleanup, nil
}
