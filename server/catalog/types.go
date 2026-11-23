package catalog

import "strings"

// Class is a coarse grouping of Go field types, used to filter options and
// Option Arguments by the field they are written on.
type Class uint16

const (
	String Class = 1 << iota
	Integer
	Float
	Bool
	Slice // slices and arrays
	Map
	Time        // time.Time
	Struct      // a struct type (named or inline)
	StructSlice // a slice/array of structs ([]Order, []*Order); combined with Slice

	Number = Integer | Float

	// Any matches every class: used for named types we can't see through
	// (other packages, or no package index).
	Any Class = 0xFFFF
)

// Resolver looks up a named type declared in the current package, reporting
// its underlying type as written ("string", "[]int"; "struct" for structs).
type Resolver func(name string) (underlying string, isStruct, ok bool)

// Classify maps a Go type as written (e.g. "*[]string", "time.Time", "Status")
// to its Class. resolve may be nil.
func Classify(typ string, resolve Resolver) Class {
	return classify(typ, resolve, 0)
}

func classify(typ string, resolve Resolver, depth int) Class {
	typ = strings.TrimLeft(typ, "*")
	switch {
	case strings.HasPrefix(typ, "["):
		c := Slice
		if i := strings.IndexByte(typ, ']'); i >= 0 && classify(typ[i+1:], resolve, depth+1) == Struct {
			c |= StructSlice
		}
		return c
	case strings.HasPrefix(typ, "map["):
		return Map
	case strings.HasPrefix(typ, "struct{"), strings.HasPrefix(typ, "struct {"):
		return Struct
	}
	switch typ {
	case "string":
		return String
	case "bool":
		return Bool
	case "int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64", "uintptr",
		"byte", "rune", "time.Duration":
		return Integer
	case "float32", "float64":
		return Float
	case "time.Time":
		return Time
	}
	if resolve != nil && depth < 8 && !strings.Contains(typ, ".") {
		if u, isStruct, ok := resolve(typ); ok {
			if isStruct {
				return Struct
			}
			return classify(u, resolve, depth+1)
		}
	}
	return Any
}

// Scalar reports whether c is a single plain Go type class
// (string, integer, float, bool, time) rather than a container or unknown type.
func (c Class) Scalar() bool {
	switch c {
	case String, Integer, Float, Bool, Time:
		return true
	}
	return false
}
