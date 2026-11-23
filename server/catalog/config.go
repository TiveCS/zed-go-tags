package catalog

import (
	"fmt"
	"sort"
	"strings"

	"go-tags-ls/naming"
)

// Config is the user's settings, passed from Zed as LSP initialization options:
//
//	"lsp": { "go-tags-ls": { "initialization_options": {
//	    "naming":      { "json": "camelCase" },
//	    "custom_keys": { "mytag": "snake_case" }
//	} } }
type Config struct {
	// Naming overrides the Naming Style of built-in Tag Keys.
	Naming map[string]naming.Style `json:"naming"`
	// CustomKeys adds Tag Keys, each with its Naming Style. They are listed
	// after the built-in keys, alphabetically.
	CustomKeys map[string]naming.Style `json:"custom_keys"`
	// TypeSearch is where other types are looked up: "package" (default) or "file".
	// Validated by the server.
	TypeSearch string `json:"type_search"`
}

// Configure returns the built-in Keys with cfg applied, plus a description of
// every setting that was ignored.
func Configure(cfg Config) (keys []Key, problems []string) {
	keys = append([]Key(nil), Keys...)
	styles := strings.Join(styleNames(), ", ")

	for _, name := range sortedNames(cfg.Naming) {
		style := cfg.Naming[name]
		i := indexOf(keys, name)
		switch {
		case i < 0:
			problems = append(problems, fmt.Sprintf("naming.%s: unknown key (add it under custom_keys instead)", name))
		case keys[i].Kind != Named:
			problems = append(problems, fmt.Sprintf("naming.%s: %s has no name to style", name, name))
		case !naming.Valid(style):
			problems = append(problems, fmt.Sprintf("naming.%s: unknown style %q (use one of: %s)", name, style, styles))
		default:
			keys[i].Style = style
		}
	}

	for _, name := range sortedNames(cfg.CustomKeys) {
		style := cfg.CustomKeys[name]
		switch {
		case !validKeyName(name):
			problems = append(problems, fmt.Sprintf("custom_keys.%q: invalid key name", name))
		case indexOf(keys, name) >= 0:
			problems = append(problems, fmt.Sprintf("custom_keys.%s: already a key (use naming.%s to change its style)", name, name))
		case !naming.Valid(style):
			problems = append(problems, fmt.Sprintf("custom_keys.%s: unknown style %q (use one of: %s)", name, style, styles))
		default:
			keys = append(keys, Key{Name: name, Library: "custom", Style: style, Seps: ","})
		}
	}
	return keys, problems
}

// Find looks a Tag Key up by name in keys.
func Find(keys []Key, name string) (Key, bool) {
	if i := indexOf(keys, name); i >= 0 {
		return keys[i], true
	}
	return Key{}, false
}

func indexOf(keys []Key, name string) int {
	for i, k := range keys {
		if k.Name == name {
			return i
		}
	}
	return -1
}

// validKeyName follows reflect.StructTag: non-empty, no space, quote, colon or control chars.
func validKeyName(name string) bool {
	if name == "" {
		return false
	}
	for _, r := range name {
		if r <= ' ' || r == '"' || r == ':' || r == '`' || r == 0x7f {
			return false
		}
	}
	return true
}

func sortedNames(m map[string]naming.Style) []string {
	names := make([]string, 0, len(m))
	for n := range m {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

func styleNames() []string {
	out := make([]string, len(naming.Styles))
	for i, s := range naming.Styles {
		out[i] = string(s)
	}
	return out
}
