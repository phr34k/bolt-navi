package ui

import (
	"regexp"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/phr34k/bolt-navi/internal/bolt"
)

var ansi = regexp.MustCompile("\x1b\\[[0-9;]*m")

func strip(s string) string { return ansi.ReplaceAllString(s, "") }

func demoModel() Model {
	m := Model{width: 80, height: 24, targetsState: "ready", pickChecked: map[int]bool{}}
	m.entry = bolt.Entry{Kind: "task", Name: "service::restart"}
	m.fields = []bolt.Field{
		{Name: "targets", Role: "targets", Widget: "text", Type: "TargetSpec", Required: true, Value: ""},
		{Name: "ensure", Role: "param", Widget: "enum", Type: "Enum[present,absent]",
			Choices: []string{"", "present", "absent"}, Desc: "desired state"},
		{Name: "force", Role: "param", Widget: "enum", Type: "Boolean",
			Choices: []string{"", "true", "false"}},
		{Name: "Run", Role: "button", Widget: "button"},
		{Name: "Cancel", Role: "button", Widget: "button"},
	}
	m.names = []string{"web1.example.com", "web2.example.com", "db1.example.com"}
	m.groups = []string{"all", "webservers"}
	return m
}

func TestRenderEnumDropdown(t *testing.T) {
	m := demoModel()
	m.mode = modeWizard
	m.fi = 1 // the "ensure" enum
	m.enumOpen = true
	m.enumIdx = 1
	out := strip(m.View())
	t.Logf("\n%s", out)
	// the "force" field sits under the dropdown: its label must survive the
	// popup (the box only covers its own columns, like Python).
	if !strings.Contains(out, "force") {
		t.Fatal("field label under the dropdown was blanked")
	}
}

func TestRenderSearchOverflowKeepsBadge(t *testing.T) {
	// go test has no TTY, so lipgloss disables color by default; force a profile
	// so Render emits escapes and we can assert the badge keeps its background.
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	m := Model{width: 40, height: 16, targetsState: "ready", pickChecked: map[int]bool{}}
	m.all = []bolt.Entry{
		{Kind: "plan", Name: "service::a_very_long_plan_name_here",
			Desc: "and an even longer description that certainly overflows the row"},
	}
	m.refilter()
	row := m.renderRow(m.results[0], false, m.width)
	// stPlan sets a background (SGR 48;5;…). It must survive the overflow.
	if !strings.Contains(row, "\x1b[") || !strings.Contains(strip(row), "PLAN") {
		t.Fatalf("badge styling lost on overflow row: %q", row)
	}
	t.Logf("plain: %q", strip(row))
	t.Logf("ansi : %q", row)
}

func TestRenderPicker(t *testing.T) {
	m := demoModel()
	m.mode = modePicker
	m.openPicker()
	m.pickIdx = 2
	m.pickChecked[2] = true
	m.pickChecked[0] = true
	t.Logf("\n%s", strip(m.View()))
}
