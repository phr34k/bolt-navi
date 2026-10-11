// Package bolt is the shared backend for the Go port of bolt-tui: it loads
// tasks/plans from bolt's on-disk caches (or live `bolt ... show`), parses the
// inventory, and builds/runs the `bolt ... run` argv. It is deliberately free
// of any UI concern so it can be unit-tested on its own.
package bolt

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Cache file names bolt keeps in the project root.
const (
	TaskCache = ".task_cache.json"
	PlanCache = ".plan_cache.json"
)

// Param is one task/plan parameter, kept in declaration order.
type Param struct {
	Name        string
	Type        string
	Description string
	Default     string // rendered default value ("" when none)
	HasDefault  bool
	Sensitive   bool
}

// Detail is the parsed metadata for a single task or plan.
type Detail struct {
	Description string
	Params      []Param // preserves the order parameters appear in the JSON
}

// Entry is one task or plan in the merged palette list.
type Entry struct {
	Kind   string // "task" | "plan"
	Name   string
	Desc   string
	Detail *Detail // nil until loaded (live fallback loads lazily)
}

// ModuleSortKey groups by module then by the short task/plan name:
// "service::restart" -> ("service", "restart"); bare "service" -> ("service", "").
func ModuleSortKey(name string) (string, string) {
	mod, rest, found := strings.Cut(name, "::")
	if !found {
		return strings.ToLower(mod), ""
	}
	return strings.ToLower(mod), strings.ToLower(rest)
}

func lessEntry(a, b Entry) bool {
	am, ar := ModuleSortKey(a.Name)
	bm, br := ModuleSortKey(b.Name)
	if am != bm {
		return am < bm
	}
	if ar != br {
		return ar < br
	}
	return a.Kind < b.Kind
}

// LoadAll returns every task and plan, preferring the on-disk caches and
// falling back to a live `bolt <kind> show` listing (in which case Detail is
// nil and the caller loads it lazily via EnsureDetail).
func LoadAll(root string) []Entry {
	var entries []Entry
	for _, kind := range []string{"task", "plan"} {
		cache, err := loadCache(kind, root)
		if err == nil && len(cache) > 0 {
			for name, raw := range cache {
				d := ParseDetail(kind, raw)
				entries = append(entries, Entry{
					Kind:   kind,
					Name:   name,
					Desc:   oneLine(d.Description),
					Detail: &d,
				})
			}
			continue
		}
		list, err := loadList(kind, root)
		if err != nil {
			continue
		}
		for _, it := range list {
			e := it
			entries = append(entries, Entry{Kind: kind, Name: e.Name, Desc: e.Desc})
		}
	}
	sort.SliceStable(entries, func(i, j int) bool { return lessEntry(entries[i], entries[j]) })
	return entries
}

// EnsureDetail loads an entry's detail live if the cache didn't provide it.
func EnsureDetail(root string, e *Entry) error {
	if e.Detail != nil {
		return nil
	}
	raw, err := loadDetail(root, e.Kind, e.Name)
	if err != nil {
		return err
	}
	d := ParseDetail(e.Kind, raw)
	e.Detail = &d
	return nil
}

// loadCache reads bolt's cache for the kind: {name: rawDetailJSON}. Private
// plans are dropped to match `bolt plan show`.
func loadCache(kind, root string) (map[string]json.RawMessage, error) {
	name := TaskCache
	if kind == "plan" {
		name = PlanCache
	}
	data, err := os.ReadFile(filepath.Join(root, name))
	if err != nil {
		return nil, err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	out := make(map[string]json.RawMessage, len(raw))
	for n, entry := range raw {
		if kind == "plan" {
			var probe struct {
				Private bool `json:"private"`
			}
			_ = json.Unmarshal(entry, &probe)
			if probe.Private {
				continue
			}
		}
		out[n] = entry
	}
	return out, nil
}

// ParseDetail extracts the description and ordered parameter list from one
// task/plan detail blob (mirrors params_of/desc_of in the Python lib).
func ParseDetail(kind string, raw json.RawMessage) Detail {
	var top map[string]json.RawMessage
	_ = json.Unmarshal(raw, &top)

	var d Detail
	if kind == "task" {
		var meta map[string]json.RawMessage
		if m, ok := top["metadata"]; ok {
			_ = json.Unmarshal(m, &meta)
		}
		d.Description = jsonString(meta["description"])
		d.Params = parseParams(meta["parameters"])
	} else {
		d.Description = jsonString(top["description"])
		if d.Description == "" {
			d.Description = jsonString(top["summary"])
		}
		d.Params = parseParams(top["parameters"])
	}
	return d
}

// parseParams decodes a parameters object while preserving key order.
func parseParams(raw json.RawMessage) []Param {
	if len(raw) == 0 {
		return nil
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	tok, err := dec.Token()
	if err != nil {
		return nil
	}
	if delim, ok := tok.(json.Delim); !ok || delim != '{' {
		return nil
	}
	var params []Param
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			break
		}
		name, _ := keyTok.(string)
		var meta struct {
			Type         string          `json:"type"`
			DefaultValue json.RawMessage `json:"default_value"`
			Description  string          `json:"description"`
			Sensitive    bool            `json:"sensitive"`
		}
		if err := dec.Decode(&meta); err != nil {
			break
		}
		p := Param{
			Name:        name,
			Type:        meta.Type,
			Description: oneLine(meta.Description),
			Sensitive:   meta.Sensitive,
		}
		if len(meta.DefaultValue) > 0 && string(meta.DefaultValue) != "null" {
			p.HasDefault = true
			p.Default = renderDefault(meta.DefaultValue)
		}
		params = append(params, p)
	}
	return params
}

// IsRequired reports whether a parameter must be supplied.
func IsRequired(kind string, p Param) bool {
	if kind == "plan" {
		return !p.HasDefault && !strings.HasPrefix(p.Type, "Optional")
	}
	return !strings.HasPrefix(p.Type, "Optional")
}

// EnumChoices parses "Enum[a, b, c]" into ["a","b","c"]; nil otherwise.
func EnumChoices(typeStr string) []string {
	t := strings.TrimSpace(typeStr)
	if !strings.HasPrefix(t, "Enum[") || !strings.HasSuffix(t, "]") {
		return nil
	}
	inner := t[len("Enum[") : len(t)-1]
	var out []string
	for _, c := range strings.Split(inner, ",") {
		c = strings.TrimSpace(c)
		c = strings.Trim(c, "'\"")
		if c != "" {
			out = append(out, c)
		}
	}
	return out
}

// IsBoolean reports whether a (possibly Optional-wrapped) type is Boolean.
func IsBoolean(typeStr string) bool {
	t := strings.TrimPrefix(typeStr, "Optional[")
	t = strings.TrimSuffix(t, "]")
	return t == "Boolean"
}

func jsonString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	return ""
}

// renderDefault produces a human string for a default value of any JSON type.
func renderDefault(raw json.RawMessage) string {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return strings.TrimSpace(string(raw))
	}
	switch t := v.(type) {
	case string:
		return t
	default:
		b, _ := json.Marshal(v)
		return string(b)
	}
}

func oneLine(s string) string {
	return strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
}
