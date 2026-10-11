package bolt

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestFuzzy(t *testing.T) {
	if _, ok := Fuzzy("xyz", "reclaim"); ok {
		t.Fatal("expected no match for xyz in reclaim")
	}
	m, ok := Fuzzy("rec", "reclaim")
	if !ok || !reflect.DeepEqual(m.Indexes, []int{0, 1, 2}) {
		t.Fatalf("contiguous prefix match wrong: %+v ok=%v", m, ok)
	}
	// A contiguous early match should outscore a later one.
	a, _ := Fuzzy("ser", "service::restart")
	b, _ := Fuzzy("ser", "user_resolver")
	if a.Score <= b.Score {
		t.Fatalf("expected contiguous-early %v to beat later %v", a.Score, b.Score)
	}
}

func TestEnumAndBoolean(t *testing.T) {
	if got := EnumChoices("Enum[present, absent, 'held']"); !reflect.DeepEqual(got, []string{"present", "absent", "held"}) {
		t.Fatalf("EnumChoices = %v", got)
	}
	if EnumChoices("String") != nil {
		t.Fatal("non-enum should be nil")
	}
	if !IsBoolean("Boolean") || !IsBoolean("Optional[Boolean]") {
		t.Fatal("Boolean detection failed")
	}
	if IsBoolean("String") {
		t.Fatal("String is not Boolean")
	}
}

func TestIsRequired(t *testing.T) {
	// task: required unless Optional[...]
	if !IsRequired("task", Param{Type: "String"}) {
		t.Fatal("task String should be required")
	}
	if IsRequired("task", Param{Type: "Optional[String]"}) {
		t.Fatal("task Optional should not be required")
	}
	// plan: not required if it has a default
	if IsRequired("plan", Param{Type: "String", HasDefault: true}) {
		t.Fatal("plan with default should not be required")
	}
	if !IsRequired("plan", Param{Type: "String"}) {
		t.Fatal("plan without default should be required")
	}
}

func TestBuildArgvTask(t *testing.T) {
	fields := []Field{
		{Name: "targets", Role: "targets", Widget: "text", Value: "web1,web2"},
		{Name: "message", Role: "param", Widget: "text", Value: "hi there"},
		{Name: "empty", Role: "param", Widget: "text", Value: ""},
		{Name: "verbose", Role: "option", Widget: "enum", Flag: "--verbose", Value: "true"},
		{Name: "run-as", Role: "option", Widget: "text", Flag: "--run-as", TakesValue: true, Value: "root"},
		{Name: "noop", Role: "option", Widget: "enum", Flag: "--noop", Value: "false"},
		{Name: "extra args", Role: "extra", Widget: "text", Value: "--log-level debug"},
	}
	got := BuildArgv("task", "service::restart", fields)
	want := []string{
		"task", "run", "service::restart",
		"message=hi there",
		"--verbose",
		"--run-as", "root",
		"--targets", "web1,web2",
		"--log-level", "debug",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("BuildArgv =\n  %#v\nwant\n  %#v", got, want)
	}
}

func TestMissingRequired(t *testing.T) {
	fields := []Field{
		{Name: "targets", Required: true, Widget: "text", Value: ""},
		{Name: "message", Required: true, Widget: "text", Value: "x"},
		{Name: "Run", Widget: "button"},
	}
	if got := MissingRequired(fields); !reflect.DeepEqual(got, []string{"targets"}) {
		t.Fatalf("MissingRequired = %v", got)
	}
}

func TestParseDetailOrder(t *testing.T) {
	raw := []byte(`{"metadata":{"description":"does a thing",
		"parameters":{
			"first":{"type":"String","description":"the first"},
			"second":{"type":"Optional[Integer]","default_value":3},
			"mode":{"type":"Enum[a,b]"}
		}}}`)
	d := ParseDetail("task", raw)
	if d.Description != "does a thing" {
		t.Fatalf("desc = %q", d.Description)
	}
	gotNames := []string{d.Params[0].Name, d.Params[1].Name, d.Params[2].Name}
	if !reflect.DeepEqual(gotNames, []string{"first", "second", "mode"}) {
		t.Fatalf("param order not preserved: %v", gotNames)
	}
	if !d.Params[1].HasDefault || d.Params[1].Default != "3" {
		t.Fatalf("second default = %q has=%v", d.Params[1].Default, d.Params[1].HasDefault)
	}
}

// TestLoadRealCaches exercises the loader against the actual project caches if
// present, so parsing is validated on real bolt output.
func TestLoadRealCaches(t *testing.T) {
	// bolt-navi is a standalone project; point the test at a real Bolt project
	// via BOLT_NAVI_TEST_PROJECT, else try the sibling "bolt" checkout.
	root := os.Getenv("BOLT_NAVI_TEST_PROJECT")
	if root == "" {
		root = filepath.Join("..", "..", "..", "bolt")
	}
	if _, err := os.Stat(filepath.Join(root, TaskCache)); err != nil {
		t.Skip("no real task cache found; set BOLT_NAVI_TEST_PROJECT to run")
	}
	entries := LoadAll(root)
	if len(entries) == 0 {
		t.Fatal("expected some tasks/plans from real caches")
	}
	for _, e := range entries {
		if e.Kind != "task" && e.Kind != "plan" {
			t.Fatalf("bad kind %q for %q", e.Kind, e.Name)
		}
		if e.Detail == nil {
			t.Fatalf("cache entry %q has nil detail", e.Name)
		}
	}
	t.Logf("loaded %d entries from real caches", len(entries))
}
