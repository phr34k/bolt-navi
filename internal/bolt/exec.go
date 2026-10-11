package bolt

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// projectDir, when set via SetProject, is forwarded to every bolt invocation
// as `--project <dir>` so bolt-navi can be installed system-wide and run
// against a project regardless of the current working directory.
var projectDir string

// SetProject records the Bolt project directory to forward to bolt. An empty
// string means "don't forward" (bolt falls back to detecting the project from
// the working directory, preserving the in-project default behavior).
func SetProject(dir string) { projectDir = dir }

// withProject appends `--project <dir>` to a bolt arg list when a project has
// been set.
func withProject(args []string) []string {
	if projectDir == "" {
		return args
	}
	out := make([]string, len(args), len(args)+2)
	copy(out, args)
	return append(out, "--project", projectDir)
}

// BoltCmd builds an *exec.Cmd for `bolt <argv...>`, wrapped for the platform
// (on Windows bolt ships as a .bat and must run through cmd /c). Its std
// streams are left nil so tea.ExecProcess can attach the terminal.
//
// The argv and the inherited environment are scrubbed of NUL bytes: on Windows
// a NUL anywhere in the command line or environment makes CreateProcess fail
// with "invalid argument" (Go's UTF16PtrFromString / createEnvBlock reject it),
// and a stray NUL can arrive from the environment or from a keypress such as
// Ctrl+Space landing in a field value.
func BoltCmd(root string, argv []string) *exec.Cmd {
	wrapped := stripNULs(boltArgv(withProject(argv)))
	cmd := exec.Command(wrapped[0], wrapped[1:]...)
	cmd.Dir = root
	cmd.Env = CleanEnviron()
	return cmd
}

// stripNULs removes NUL bytes from every element of argv.
func stripNULs(argv []string) []string {
	out := make([]string, len(argv))
	for i, a := range argv {
		out[i] = strings.ReplaceAll(a, "\x00", "")
	}
	return out
}

// CleanEnviron returns os.Environ() with any NUL-containing entries dropped, so
// it is always safe to hand to exec on Windows.
func CleanEnviron() []string {
	env := os.Environ()
	out := make([]string, 0, len(env))
	for _, e := range env {
		if !strings.ContainsRune(e, 0) {
			out = append(out, e)
		}
	}
	return out
}

// loadList runs `bolt <kind> show --format json` and returns name/desc pairs.
// Used only when the on-disk cache is unavailable.
func loadList(kind, root string) ([]Entry, error) {
	var data map[string]json.RawMessage
	if err := runBoltJSON(root, []string{kind, "show", "--format", "json"}, &data); err != nil {
		return nil, err
	}
	key := "tasks"
	if kind == "plan" {
		key = "plans"
	}
	var rows [][]any
	if raw, ok := data[key]; ok {
		_ = json.Unmarshal(raw, &rows)
	}
	var out []Entry
	for _, row := range rows {
		if len(row) == 0 {
			continue
		}
		name, _ := row[0].(string)
		desc := ""
		if len(row) > 1 {
			desc, _ = row[1].(string)
		}
		out = append(out, Entry{Kind: kind, Name: name, Desc: oneLine(desc)})
	}
	return out, nil
}

func loadDetail(root, kind, name string) (json.RawMessage, error) {
	var raw json.RawMessage
	if err := runBoltJSON(root, []string{kind, "show", name, "--format", "json"}, &raw); err != nil {
		return nil, err
	}
	return raw, nil
}

// ModuleInstall runs `bolt module install` silently in the background so the
// task/plan cache is refreshed on startup without blocking the TUI.
func ModuleInstall(root string) error {
	wrapped := boltArgv(withProject([]string{"module", "install"}))
	cmd := exec.Command(wrapped[0], wrapped[1:]...)
	cmd.Dir = root
	cmd.Env = CleanEnviron()
	return cmd.Run()
}

// runBoltJSON runs bolt capturing stdout and decodes it into v.
func runBoltJSON(root string, argv []string, v any) error {
	wrapped := boltArgv(withProject(argv))
	cmd := exec.Command(wrapped[0], wrapped[1:]...)
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok && len(ee.Stderr) > 0 {
			return fmt.Errorf("bolt: %s", ee.Stderr)
		}
		return err
	}
	if err := json.Unmarshal(out, v); err != nil {
		return fmt.Errorf("could not parse bolt JSON: %w", err)
	}
	return nil
}
