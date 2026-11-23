// Package tagctx works out where the cursor sits inside a Go Struct Tag.
package tagctx

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"strings"

	"go-tags-ls/pkgindex"
)

// Position is the part of a Struct Tag the cursor is in.
type Position int

const (
	None  Position = iota // inside a tag, but nowhere a suggestion makes sense (e.g. right after `key:`)
	Key                   // where a Tag Key goes
	Value                 // inside the quotes of a Tag Key's value
)

func (p Position) String() string {
	return [...]string{"None", "Key", "Value"}[p]
}

// Context describes the Struct Tag under the cursor.
type Context struct {
	Position Position

	// FieldNames are the declared field names; for an embedded field it is
	// the embedded type's name. Grouped fields (`A, B string`) list all names.
	FieldNames []string
	FieldType  string // Go type as written, e.g. "string", "*time.Time", "[]int"
	Embedded   bool
	// StructName is the enclosing struct's type name ("" for an anonymous
	// struct or when the file doesn't parse).
	StructName string
	// SiblingFields are the names of the other fields in the same struct,
	// in declaration order. Empty when the file doesn't parse.
	SiblingFields []string

	// KeyPartial is the Tag Key typed so far (Position == Key).
	KeyPartial string
	// Key and ValuePrefix are the owning Tag Key and the raw text between its
	// opening quote and the cursor (Position == Value).
	Key         string
	ValuePrefix string

	// ExistingKeys are Tag Keys already present in the tag, excluding the
	// one currently being typed at Position == Key.
	ExistingKeys []string
}

// At reports the Struct Tag context at the given 0-based line and byte column
// of src. ok is false when the cursor is not inside a struct field's tag.
func At(src string, line, col int) (ctx Context, ok bool) {
	lines := strings.Split(src, "\n")
	if line < 0 || line >= len(lines) {
		return Context{}, false
	}
	text := strings.TrimSuffix(lines[line], "\r")
	if col < 0 || col > len(text) {
		return Context{}, false
	}

	// The tag opens at the first backtick on the line and closes at the next.
	open := strings.IndexByte(text, '`')
	if open < 0 || col <= open {
		return Context{}, false
	}
	closeIdx := len(text) // unclosed while typing
	closed := false
	if j := strings.IndexByte(text[open+1:], '`'); j >= 0 {
		closeIdx = open + 1 + j
		closed = true
	}
	if col > closeIdx {
		return Context{}, false
	}

	field, ok := findField(lines, line, open, closed)
	if !ok {
		return Context{}, false
	}
	ctx = field

	content := text[open+1 : closeIdx]
	cur := scan(content[:col-open-1])
	all := scan(content)

	skipKeyAt := -1
	switch cur.state {
	case stBetween:
		ctx.Position = Key
	case stKey:
		ctx.Position = Key
		ctx.KeyPartial = content[cur.keyStart : col-open-1]
		skipKeyAt = cur.keyStart
	case stValue:
		ctx.Position = Value
		ctx.Key = cur.key
		ctx.ValuePrefix = content[cur.valStart : col-open-1]
	default:
		ctx.Position = None
	}
	for _, p := range all.pairs {
		if p.keyStart != skipKeyAt {
			ctx.ExistingKeys = append(ctx.ExistingKeys, p.key)
		}
	}
	return ctx, true
}

// findField identifies the struct field declared on lines[line], whose tag
// opens at byte column open. It first parses the whole file (with the tag
// closed if needed); if the file has other syntax errors it falls back to
// parsing the single line inside a synthetic struct.
func findField(lines []string, line, open int, closed bool) (Context, bool) {
	fixed := lines
	if !closed {
		fixed = append([]string(nil), lines...)
		fixed[line] = strings.TrimSuffix(fixed[line], "\r") + "`"
	}
	src := strings.Join(fixed, "\n")

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "", src, parser.SkipObjectResolution)
	if err == nil {
		return fieldAt(fset, file, line+1, open+1)
	}

	// Whole file is broken elsewhere: judge this line on its own.
	decl := strings.TrimSpace(fixed[line][:open])
	if decl == "" {
		return Context{}, false
	}
	synthetic := "package p\ntype _ struct {\n" + decl + " ``\n}\n"
	fset = token.NewFileSet()
	file, err = parser.ParseFile(fset, "", synthetic, parser.SkipObjectResolution)
	if err != nil {
		return Context{}, false
	}
	var found Context
	var ok bool
	ast.Inspect(file, func(n ast.Node) bool {
		if st, isStruct := n.(*ast.StructType); isStruct && len(st.Fields.List) == 1 {
			found, ok = describe(st.Fields.List[0]), true
			return false
		}
		return true
	})
	return found, ok
}

// fieldAt finds a struct field whose tag literal starts at line:col (1-based).
func fieldAt(fset *token.FileSet, file *ast.File, line, col int) (Context, bool) {
	names := map[*ast.StructType]string{}
	ast.Inspect(file, func(n ast.Node) bool {
		if ts, isSpec := n.(*ast.TypeSpec); isSpec {
			if st, isStruct := ts.Type.(*ast.StructType); isStruct {
				names[st] = ts.Name.Name
			}
		}
		return true
	})

	var found Context
	var ok bool
	ast.Inspect(file, func(n ast.Node) bool {
		if ok {
			return false
		}
		st, isStruct := n.(*ast.StructType)
		if !isStruct {
			return true
		}
		for _, f := range st.Fields.List {
			if f.Tag == nil {
				continue
			}
			p := fset.Position(f.Tag.Pos())
			if p.Line == line && p.Column == col {
				found, ok = describe(f), true
				found.StructName = names[st]
				for _, other := range st.Fields.List {
					if other != f {
						found.SiblingFields = append(found.SiblingFields, describe(other).FieldNames...)
					}
				}
				return false
			}
		}
		return true
	})
	return found, ok
}

func describe(f *ast.Field) Context {
	ctx := Context{FieldType: types.ExprString(f.Type)}
	if len(f.Names) == 0 {
		ctx.Embedded = true
		ctx.FieldNames = []string{pkgindex.EmbeddedName(f.Type)}
		return ctx
	}
	for _, n := range f.Names {
		ctx.FieldNames = append(ctx.FieldNames, n.Name)
	}
	return ctx
}
