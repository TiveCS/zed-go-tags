// Package catalog defines the known Tag Keys: their order, library,
// default Naming Style, how their Tag Value is shaped and their Tag Options.
package catalog

import (
	"strings"

	"go-tags-ls/naming"
)

// Kind says what a Tag Key's value holds.
type Kind int

const (
	Named Kind = iota // value starts with a name derived from the field (json:"user_id")
	Rules             // value holds only rules/options (validate:"required,email")
)

// Key is one known Tag Key.
type Key struct {
	Name    string
	Library string
	Kind    Kind
	Style   naming.Style // Named keys only
	// Prefix goes before the derived name, e.g. gorm's "column:". Keys with a
	// Prefix treat the name as just another ;-part rather than the first part.
	Prefix string
	// Seps are the characters separating parts of the value.
	Seps string
	// SpaceAfterSep inserts ", " instead of "," before an option (go-envconfig style).
	SpaceAfterSep bool
	// Skip means "-" is a valid name, telling the library to ignore the field.
	Skip bool
	// FoldCase means option names are case-insensitive (gorm).
	FoldCase bool
	Options  []Option
}

// Keys in popularity order; this is the order shown in the popup.
var Keys = []Key{
	{Name: "json", Library: "encoding/json", Style: naming.Snake, Seps: ",", Skip: true, Options: jsonOptions},
	{Name: "validate", Library: "go-playground/validator", Kind: Rules, Seps: ",|", Options: validateOptions},
	{Name: "query", Library: "echo", Style: naming.Snake, Seps: ","},
	{Name: "param", Library: "echo", Style: naming.Snake, Seps: ","},
	{Name: "form", Library: "echo", Style: naming.Snake, Seps: ","},
	{Name: "header", Library: "echo", Style: naming.Header, Seps: ","},
	{Name: "db", Library: "sqlx", Style: naming.Snake, Seps: ",", Skip: true},
	{Name: "gorm", Library: "gorm", Style: naming.Snake, Prefix: "column:", Seps: ";", FoldCase: true, Options: gormOptions},
	{Name: "env", Library: "sethvargo/go-envconfig", Style: naming.ScreamingSnake, Seps: ",", SpaceAfterSep: true, Options: envOptions},
	{Name: "yaml", Library: "yaml", Style: naming.Snake, Seps: ",", Skip: true, Options: yamlOptions},
	{Name: "xml", Library: "encoding/xml", Style: naming.Camel, Seps: ",", Skip: true, Options: xmlOptions},
	{Name: "toml", Library: "toml", Style: naming.Snake, Seps: ",", Skip: true, Options: tomlOptions},
	{Name: "mapstructure", Library: "mapstructure", Style: naming.Snake, Seps: ",", Skip: true, Options: mapstructureOptions},
	{Name: "bson", Library: "mongo-driver", Style: naming.Camel, Seps: ",", Skip: true, Options: bsonOptions},
}

// Value is the suggested Tag Value for a field name, e.g. "user_id" or "column:user_id".
func (k Key) Value(fieldName string) string {
	return k.Prefix + naming.Apply(k.Style, fieldName)
}

// Split cuts the value text before the cursor into its parts. The last part
// is the one being typed; it may be "" (cursor right after a separator).
func (k Key) Split(valuePrefix string) []string {
	var parts []string
	start := 0
	for i := 0; i < len(valuePrefix); i++ {
		if strings.IndexByte(k.Seps, valuePrefix[i]) >= 0 {
			parts = append(parts, valuePrefix[start:i])
			start = i + 1
		}
	}
	return append(parts, valuePrefix[start:])
}

// optionName is the comparable name of a typed part: "min=5" -> "min=", "size:255" -> "size:".
func optionName(part string) string {
	part = strings.TrimSpace(part)
	if i := strings.IndexAny(part, "=:"); i >= 0 {
		return part[:i+1]
	}
	return part
}

// Used reports whether option o already appears in parts.
func (k Key) Used(o Option, parts []string) bool {
	for _, p := range parts {
		n := optionName(p)
		if !o.TakesArg() {
			n = strings.TrimRight(n, "=:") // gorm "<-:create" uses "<-"
		}
		if n == o.Name || (k.FoldCase && strings.EqualFold(n, o.Name)) {
			return true
		}
	}
	return false
}

// ArgOption finds the option whose Option Argument is being typed in part
// (e.g. "type:uu" -> type:, "uu"; gorm "<-:cr" -> <-, "cr").
func (k Key) ArgOption(part string) (Option, string, bool) {
	i := strings.IndexAny(part, "=:")
	if i < 0 {
		return Option{}, "", false
	}
	name, bare := part[:i+1], part[:i]
	for _, o := range k.Options {
		match := o.Name == name || (k.FoldCase && strings.EqualFold(o.Name, name))
		if !match && !o.TakesArg() && len(o.Args) > 0 && part[i] == ':' {
			match = o.Name == bare
		}
		if match {
			return o, part[i+1:], true
		}
	}
	return Option{}, "", false
}

// Allows reports whether an argument fits a field of class c. Fields whose
// type isn't a plain scalar (slices, maps, named types) see every argument.
func (a Arg) Allows(c Class) bool {
	return a.For == 0 || !c.Scalar() || a.For&c != 0
}
