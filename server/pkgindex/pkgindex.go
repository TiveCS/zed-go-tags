// Package pkgindex collects the types and typed constants declared in a Go
// package, so completions can see structs and constants outside the current
// struct (related gorm models, enum-like constants for validate oneof=).
package pkgindex

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"strconv"
	"strings"
)

// Type is a named type declared in the package.
type Type struct {
	Struct bool
	// Underlying is the declared underlying type as written, e.g. "string",
	// "[]int"; "struct" for struct types.
	Underlying string
	// Fields are a struct's field names (embedded fields by type name), in order.
	Fields []string
}

// Const is a typed constant, e.g. `StatusActive Status = "active"`.
type Const struct {
	Name  string
	Value string // unquoted for strings, decimal for numbers
}

// File is what one source file contributes to the index.
type File struct {
	Package string
	Types   map[string]Type
	Consts  map[string][]Const // by type name, in declaration order
}

// ParseFile indexes one source file. It tolerates syntax errors: whatever
// the parser recovers is kept.
func ParseFile(src string) *File {
	f := &File{Types: map[string]Type{}, Consts: map[string][]Const{}}
	file, _ := parser.ParseFile(token.NewFileSet(), "", src, parser.SkipObjectResolution)
	if file == nil {
		return f
	}
	if file.Name != nil {
		f.Package = file.Name.Name
	}
	for _, decl := range file.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		switch gd.Tok {
		case token.TYPE:
			for _, spec := range gd.Specs {
				ts := spec.(*ast.TypeSpec)
				f.Types[ts.Name.Name] = describeType(ts)
			}
		case token.CONST:
			collectConsts(gd, f.Consts)
		}
	}
	return f
}

func describeType(ts *ast.TypeSpec) Type {
	st, ok := ts.Type.(*ast.StructType)
	if !ok {
		return Type{Underlying: types.ExprString(ts.Type)}
	}
	t := Type{Struct: true, Underlying: "struct"}
	for _, field := range st.Fields.List {
		if len(field.Names) == 0 {
			t.Fields = append(t.Fields, EmbeddedName(field.Type))
			continue
		}
		for _, n := range field.Names {
			t.Fields = append(t.Fields, n.Name)
		}
	}
	return t
}

// EmbeddedName is the implicit field name of an embedded type: *pkg.Base -> Base.
func EmbeddedName(expr ast.Expr) string {
	for {
		switch e := expr.(type) {
		case *ast.StarExpr:
			expr = e.X
		case *ast.SelectorExpr:
			return e.Sel.Name
		case *ast.IndexExpr:
			expr = e.X
		case *ast.IndexListExpr:
			expr = e.X
		case *ast.Ident:
			return e.Name
		default:
			return types.ExprString(expr)
		}
	}
}

// collectConsts records typed constants of a const block, following Go's
// implicit repetition (a spec with no type and no values repeats the
// previous one) and evaluating simple iota expressions.
func collectConsts(gd *ast.GenDecl, out map[string][]Const) {
	var typ ast.Expr
	var values []ast.Expr
	for iota, spec := range gd.Specs {
		vs := spec.(*ast.ValueSpec)
		if vs.Type != nil || len(vs.Values) > 0 {
			typ, values = vs.Type, vs.Values
		}
		ident, ok := typ.(*ast.Ident)
		if !ok {
			continue // untyped or qualified type: not an enum of a local type
		}
		for i, name := range vs.Names {
			if name.Name == "_" || i >= len(values) {
				continue
			}
			if v, ok := evalConst(values[i], iota); ok {
				out[ident.Name] = append(out[ident.Name], Const{Name: name.Name, Value: v})
			}
		}
	}
}

// evalConst evaluates string/number literals, iota, and iota ± n / n + iota.
func evalConst(e ast.Expr, iota int) (string, bool) {
	switch e := e.(type) {
	case *ast.BasicLit:
		switch e.Kind {
		case token.STRING:
			s, err := strconv.Unquote(e.Value)
			return s, err == nil
		case token.INT, token.FLOAT:
			return e.Value, true
		}
	case *ast.Ident:
		if e.Name == "iota" {
			return strconv.Itoa(iota), true
		}
	case *ast.ParenExpr:
		return evalConst(e.X, iota)
	case *ast.UnaryExpr:
		if v, ok := evalInt(e.X, iota); ok && e.Op == token.SUB {
			return strconv.Itoa(-v), true
		}
	case *ast.BinaryExpr:
		x, okx := evalInt(e.X, iota)
		y, oky := evalInt(e.Y, iota)
		if !okx || !oky {
			return "", false
		}
		switch e.Op {
		case token.ADD:
			return strconv.Itoa(x + y), true
		case token.SUB:
			return strconv.Itoa(x - y), true
		case token.MUL:
			return strconv.Itoa(x * y), true
		}
	}
	return "", false
}

func evalInt(e ast.Expr, iota int) (int, bool) {
	s, ok := evalConst(e, iota)
	if !ok {
		return 0, false
	}
	n, err := strconv.Atoi(s)
	return n, err == nil
}

// Index is the merged view of a package's files.
type Index struct {
	Types  map[string]Type
	Consts map[string][]Const
}

// Merge combines files belonging to package pkg; files of other packages
// in the same folder (e.g. pkg_test) are skipped.
func Merge(pkg string, files []*File) *Index {
	idx := &Index{Types: map[string]Type{}, Consts: map[string][]Const{}}
	for _, f := range files {
		if f == nil || f.Package != pkg {
			continue
		}
		for name, t := range f.Types {
			idx.Types[name] = t
		}
		for name, cs := range f.Consts {
			idx.Consts[name] = append(idx.Consts[name], cs...)
		}
	}
	return idx
}

// Resolve reports a local named type's underlying type ("struct" for structs).
// It is nil-safe so callers can pass a missing index.
func (idx *Index) Resolve(name string) (underlying string, isStruct, ok bool) {
	if idx == nil {
		return "", false, false
	}
	t, ok := idx.Types[name]
	return t.Underlying, t.Struct, ok
}

// Struct returns a local struct type's fields.
func (idx *Index) Struct(name string) ([]string, bool) {
	if idx == nil {
		return nil, false
	}
	t, ok := idx.Types[name]
	if !ok || !t.Struct {
		return nil, false
	}
	return t.Fields, true
}

// ElemTypeName is the local type name a field type refers to, looking
// through pointers, slices and arrays: "[]*Order" -> "Order". It returns ""
// for qualified (other-package) and composite types.
func ElemTypeName(typ string) string {
	for {
		typ = strings.TrimLeft(typ, "*")
		if !strings.HasPrefix(typ, "[") {
			break
		}
		i := strings.IndexByte(typ, ']')
		if i < 0 {
			return ""
		}
		typ = typ[i+1:]
	}
	if typ == "" || !isIdent(typ) {
		return ""
	}
	return typ
}

func isIdent(s string) bool {
	for i, r := range s {
		if r == '_' || ('a' <= r && r <= 'z') || ('A' <= r && r <= 'Z') || (i > 0 && '0' <= r && r <= '9') || r > 127 {
			continue
		}
		return false
	}
	return true
}
