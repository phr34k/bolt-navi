package bolt

import "strings"

// Field is one editable row in the run form. Buttons (Run/Cancel) are appended
// by the UI, not by BuildFields.
type Field struct {
	Name       string
	Role       string // "targets" | "param" | "option" | "extra" | "button"
	Widget     string // "text" | "enum" | "button"
	Value      string
	Type       string
	Required   bool
	Desc       string
	Default    string
	Sensitive  bool
	Choices    []string // enum options (first entry is "" for unset)
	CI         int      // index into Choices for the current enum selection
	Flag       string   // option flag, e.g. "--verbose"
	TakesValue bool     // option takes a value vs. being a boolean flag
}

// commonOption describes a bolt CLI flag surfaced as a form field.
type commonOption struct {
	name       string
	flag       string
	takesValue bool
	desc       string
}

var commonOptions = []commonOption{
	{"run-as", "--run-as", true, "Run as another user (sudo)."},
	{"concurrency", "--concurrency", true, "Max simultaneous connections."},
	{"verbose", "--verbose", false, "Verbose logging."},
	{"stream", "--stream", false, "Stream output as it is produced."},
	{"tty", "--tty", false, "Allocate a pseudo-tty."},
	{"description", "--description", true, "Description recorded for this run."},
}

var taskOnlyOptions = []commonOption{
	{"noop", "--noop", false, "Dry run -- make no changes."},
}

func optionsFor(kind string) []commonOption {
	if kind == "task" {
		return append(append([]commonOption{}, taskOnlyOptions...), commonOptions...)
	}
	return commonOptions
}

func optionField(o commonOption) Field {
	if o.takesValue {
		return Field{
			Name: o.name, Role: "option", Widget: "text", Type: "flag value",
			Flag: o.flag, TakesValue: true, Desc: o.desc,
		}
	}
	return Field{
		Name: o.name, Role: "option", Widget: "enum", Type: "boolean flag",
		Flag: o.flag, TakesValue: false, Desc: o.desc,
		Choices: []string{"", "true", "false"},
	}
}

// BuildFields produces the editable field list for a task/plan.
func BuildFields(kind string, d *Detail) []Field {
	var fields []Field
	if kind == "task" {
		fields = append(fields, Field{
			Name: "targets", Role: "targets", Widget: "text", Type: "TargetSpec",
			Required: true,
			Desc:     "Comma-separated targets or groups (--targets).",
		})
	}
	if d != nil {
		for _, p := range d.Params {
			widget := "text"
			var choices []string
			if ch := EnumChoices(p.Type); ch != nil {
				widget = "enum"
				choices = append([]string{""}, ch...)
			} else if IsBoolean(p.Type) {
				widget = "enum"
				choices = []string{"", "true", "false"}
			}
			role := "param"
			if p.Name == "targets" || p.Name == "nodes" {
				role = "targets"
			}
			fields = append(fields, Field{
				Name: p.Name, Role: role, Widget: widget, Type: p.Type,
				Required: IsRequired(kind, p), Desc: p.Description,
				Default: p.Default, Sensitive: p.Sensitive, Choices: choices,
			})
		}
	}
	for _, o := range optionsFor(kind) {
		fields = append(fields, optionField(o))
	}
	fields = append(fields, Field{
		Name: "extra args", Role: "extra", Widget: "text", Type: "flags",
		Desc: "Any other raw bolt flags not listed above.",
	})
	return fields
}

// BuildArgv composes the `bolt <kind> run <name> ...` argument list.
func BuildArgv(kind, name string, fields []Field) []string {
	argv := []string{kind, "run", name}
	var targets, extra string
	for _, f := range fields {
		if f.Widget == "button" {
			continue
		}
		if f.Role == "extra" {
			extra = f.Value
			continue
		}
		v := strings.TrimSpace(f.Value)
		if f.Role == "option" {
			if f.TakesValue {
				if v != "" {
					argv = append(argv, f.Flag, v)
				}
			} else if v == "true" {
				argv = append(argv, f.Flag)
			}
			continue
		}
		if f.Role == "targets" && kind == "task" {
			targets = v
			continue
		}
		if v == "" {
			continue
		}
		argv = append(argv, f.Name+"="+v)
	}
	if kind == "task" && targets != "" {
		argv = append(argv, "--targets", targets)
	}
	if strings.TrimSpace(extra) != "" {
		argv = append(argv, splitArgs(extra)...)
	}
	return argv
}

// MissingRequired returns the names of required fields left empty.
func MissingRequired(fields []Field) []string {
	var miss []string
	for _, f := range fields {
		if f.Widget != "button" && f.Required && strings.TrimSpace(f.Value) == "" {
			miss = append(miss, f.Name)
		}
	}
	return miss
}

// Preview renders the argv as a copy-pasteable command line, quoted for the
// current platform (see preview_windows.go / preview_unix.go).
func Preview(argv []string) string {
	return "bolt " + quoteArgs(argv)
}

// splitArgs does a best-effort shell split of free-form extra flags, falling
// back to whitespace splitting on an unbalanced quote (mirrors the Python lib).
func splitArgs(s string) []string {
	fields, ok := shlexSplit(s)
	if !ok {
		return strings.Fields(s)
	}
	return fields
}

// shlexSplit is a minimal POSIX-ish tokenizer supporting single/double quotes
// and backslash escapes. ok=false signals an unterminated quote.
func shlexSplit(s string) ([]string, bool) {
	var out []string
	var cur strings.Builder
	inToken := false
	var quote rune
	escaped := false
	for _, r := range s {
		switch {
		case escaped:
			cur.WriteRune(r)
			escaped = false
			inToken = true
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				cur.WriteRune(r)
			}
		case r == '\\':
			escaped = true
			inToken = true
		case r == '\'' || r == '"':
			quote = r
			inToken = true
		case r == ' ' || r == '\t' || r == '\n':
			if inToken {
				out = append(out, cur.String())
				cur.Reset()
				inToken = false
			}
		default:
			cur.WriteRune(r)
			inToken = true
		}
	}
	if quote != 0 || escaped {
		return nil, false
	}
	if inToken {
		out = append(out, cur.String())
	}
	return out, true
}
