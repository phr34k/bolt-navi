package bolt

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// LoadTargetsFast reads target & group names straight from inventory.yaml
// (near-instant: no `bolt inventory show` Ruby/Vault spawn). It falls back to
// the authoritative bolt-based loader only if the file is missing, unparseable,
// or empty.
func LoadTargetsFast(root string) (names, groups []string) {
	data, err := os.ReadFile(filepath.Join(root, "inventory.yaml"))
	if err == nil {
		var inv map[string]any
		if yaml.Unmarshal(data, &inv) == nil {
			names, groups = targetsFromInventory(inv)
			if len(names) > 0 || len(groups) > 0 {
				return names, groups
			}
		}
	}
	return LoadTargets(root)
}

// LoadTargets is the authoritative loader: it asks bolt, then falls back to a
// direct inventory.yaml parse. Best-effort; never panics.
func LoadTargets(root string) (names, groups []string) {
	var data struct {
		Targets []any `json:"targets"`
		Groups  []any `json:"groups"`
	}
	if err := runBoltJSON(root, []string{"inventory", "show", "--targets", "--format", "json"}, &data); err == nil {
		seenN := map[string]bool{}
		for _, t := range data.Targets {
			if n := nameOf(t); n != "" && !seenN[n] {
				seenN[n] = true
				names = append(names, n)
			}
		}
		seenG := map[string]bool{}
		for _, g := range data.Groups {
			if n := nameOf(g); n != "" && !seenG[n] {
				seenG[n] = true
				groups = append(groups, n)
			}
		}
		if len(names) > 0 || len(groups) > 0 {
			return names, groups
		}
	}

	raw, err := os.ReadFile(filepath.Join(root, "inventory.yaml"))
	if err != nil {
		return names, groups
	}
	var inv map[string]any
	if yaml.Unmarshal(raw, &inv) != nil {
		return names, groups
	}
	return targetsFromInventory(inv)
}

// targetsFromInventory walks a parsed inventory, collecting target and group
// names from the nested groups/targets tree.
func targetsFromInventory(inv map[string]any) (names, groups []string) {
	seenN := map[string]bool{}
	seenG := map[string]bool{}

	var walkTargets func(v any)
	var walkGroups func(v any)

	addName := func(n string) {
		if n != "" && !seenN[n] {
			seenN[n] = true
			names = append(names, n)
		}
	}

	walkTargets = func(v any) {
		lst, _ := v.([]any)
		for _, t := range lst {
			addName(nameOf(t))
		}
	}
	walkGroups = func(v any) {
		lst, _ := v.([]any)
		for _, g := range lst {
			gm, ok := g.(map[string]any)
			if !ok {
				continue
			}
			if gn, _ := gm["name"].(string); gn != "" && !seenG[gn] {
				seenG[gn] = true
				groups = append(groups, gn)
			}
			walkTargets(gm["targets"])
			walkGroups(gm["groups"])
		}
	}

	walkTargets(inv["targets"])
	walkGroups(inv["groups"])
	return names, groups
}

// nameOf extracts a target/group name from a string or a {name|uri} map.
func nameOf(t any) string {
	switch v := t.(type) {
	case string:
		return v
	case map[string]any:
		if n, ok := v["name"].(string); ok && n != "" {
			return n
		}
		if u, ok := v["uri"].(string); ok {
			return u
		}
	}
	return ""
}
