package bolt

import (
	"reflect"
	"strings"
	"testing"
)

// A NUL byte anywhere in the command line makes Windows CreateProcess fail with
// "invalid argument"; BoltCmd must scrub argv so a stray NUL (from the env or a
// Ctrl+Space keypress landing in a field) can never reach process creation.
func TestBoltCmdStripsNULs(t *testing.T) {
	c := BoltCmd(".", []string{"plan", "run", "facts::info", "targets=web1\x00web2"})
	for _, a := range c.Args {
		if strings.ContainsRune(a, 0) {
			t.Fatalf("NUL survived in arg %q", a)
		}
	}
	// the meaningful text must still be there, just without the NUL
	joined := strings.Join(c.Args, " ")
	if !strings.Contains(joined, "targets=web1web2") {
		t.Fatalf("arg content mangled: %q", joined)
	}
}

func TestProjectForwarding(t *testing.T) {
	defer SetProject("") // global; don't leak into other tests

	// no project set -> nothing forwarded
	SetProject("")
	if got := withProject([]string{"task", "run", "x"}); len(got) != 3 {
		t.Fatalf("unset project should not add args, got %v", got)
	}

	// project set -> appended as --project <dir>, original slice untouched
	SetProject(`C:\ops\bolt`)
	orig := []string{"task", "run", "x"}
	got := withProject(orig)
	want := []string{"task", "run", "x", "--project", `C:\ops\bolt`}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("withProject = %v, want %v", got, want)
	}
	if len(orig) != 3 {
		t.Fatalf("withProject mutated its input: %v", orig)
	}

	// it reaches the actual command line bolt is launched with
	joined := strings.Join(BoltCmd(".", []string{"task", "run", "x"}).Args, " ")
	if !strings.Contains(joined, `--project C:\ops\bolt`) {
		t.Fatalf("BoltCmd did not forward --project: %q", joined)
	}
}

func TestCleanEnvironDropsNUL(t *testing.T) {
	t.Setenv("BOLT_TUI_OK", "fine")
	for _, e := range CleanEnviron() {
		if strings.ContainsRune(e, 0) {
			t.Fatalf("CleanEnviron returned a NUL entry: %q", e)
		}
	}
}
