package catalog

import "strings"

// Class is a coarse grouping of Go field types, used to filter validate rules.
type Class uint8

const (
	String Class = 1 << iota
	Integer
	Float
	Bool
	Slice // slices and arrays
	Map
	Time // time.Time

	Number = Integer | Float

	// Any matches every class: used for rules that apply to all types,
	// and for named types we can't see through (e.g. type Status string).
	Any Class = 0xFF
)

// Classify maps a Go type as written (e.g. "*[]string", "time.Time") to its Class.
func Classify(typ string) Class {
	typ = strings.TrimLeft(typ, "*")
	switch {
	case strings.HasPrefix(typ, "["):
		return Slice
	case strings.HasPrefix(typ, "map["):
		return Map
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
