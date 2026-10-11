package bolt

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ShellCmd builds an interactive shell to drop into from the TUI, preferring
// PowerShell. On exit the shell writes its final environment to dumpPath (via a
// generated .ps1 that hooks PowerShell.Exiting) so MergeEnvDump can carry
// changes back. cleanup removes the temp files. dumpPath is "" when the shell
// cannot capture its environment (plain cmd.exe), signalling "no merge".
func ShellCmd() (cmd *exec.Cmd, dumpPath string, cleanup func(), err error) {
	dump, err := os.CreateTemp("", "bolt_tui_env_*.txt")
	if err != nil {
		return nil, "", func() {}, err
	}
	dumpPath = dump.Name()
	dump.Close()

	exe := pickShell()
	low := strings.ToLower(filepath.Base(exe))
	env := append(CleanEnviron(), envDumpVar+"="+dumpPath)

	if strings.Contains(low, "pwsh") || strings.Contains(low, "powershell") {
		ps, perr := os.CreateTemp("", "bolt_tui_*.ps1")
		if perr != nil {
			os.Remove(dumpPath)
			return nil, "", func() {}, perr
		}
		fmt.Fprintf(ps,
			"if (Test-Path $PROFILE) { . $PROFILE }\n"+
				"Register-EngineEvent -SourceIdentifier PowerShell.Exiting -Action {\n"+
				"  Get-ChildItem Env: | ForEach-Object { \"$($_.Name)=$($_.Value)\" } |\n"+
				"    Set-Content -LiteralPath $Env:%s -Encoding utf8\n"+
				"} | Out-Null\n", envDumpVar)
		psName := ps.Name()
		ps.Close()

		cmd = exec.Command(exe, "-NoLogo", "-NoExit", "-File", psName)
		cmd.Env = env
		cleanup = func() { os.Remove(dumpPath); os.Remove(psName) }
		return cmd, dumpPath, cleanup, nil
	}

	// cmd.exe (or anything else): run it, but we can't capture its environment.
	cmd = exec.Command(exe)
	cmd.Env = env
	cleanup = func() { os.Remove(dumpPath) }
	return cmd, "", cleanup, nil
}

func pickShell() string {
	if p, err := exec.LookPath("pwsh"); err == nil {
		return p
	}
	if p, err := exec.LookPath("powershell"); err == nil {
		return p
	}
	if c := os.Getenv("COMSPEC"); c != "" {
		return c
	}
	return "cmd.exe"
}
