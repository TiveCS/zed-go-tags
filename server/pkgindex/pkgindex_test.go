package pkgindex

import (
	"reflect"
	"testing"
)

const models = `package models

import "time"

type Status string

const (
	StatusActive   Status = "active"
	StatusInactive Status = "inactive"
	statusHidden   Status = "hidden"
	_              Status = "skipped"
)

type Level int

const (
	LevelLow Level = iota + 1
	LevelMid
	LevelHigh
)

type Weekday int

const (
	Sunday Weekday = iota
	Monday
)

const Untyped = "x"

const Remote time.Duration = 5

type Customer struct {
	Base
	ID     uint
	Name, Nick string
	Orders []Order
}

type ID = string
`

func TestParseFile(t *testing.T) {
	f := ParseFile(models)
	if f.Package != "models" {
		t.Fatalf("package = %q", f.Package)
	}
	if got := f.Types["Customer"]; !got.Struct || !reflect.DeepEqual(got.Fields, []string{"Base", "ID", "Name", "Nick", "Orders"}) {
		t.Errorf("Customer = %+v", got)
	}
	if got := f.Types["Status"]; got.Struct || got.Underlying != "string" {
		t.Errorf("Status = %+v", got)
	}
	if got := f.Types["ID"]; got.Underlying != "string" {
		t.Errorf("alias ID = %+v", got)
	}

	values := func(typ string) []string {
		var out []string
		for _, c := range f.Consts[typ] {
			out = append(out, c.Value)
		}
		return out
	}
	tests := map[string][]string{
		"Status":  {"active", "inactive", "hidden"},
		"Level":   {"1", "2", "3"},
		"Weekday": {"0", "1"},
	}
	for typ, want := range tests {
		if got := values(typ); !reflect.DeepEqual(got, want) {
			t.Errorf("consts %s = %v, want %v", typ, got, want)
		}
	}
	if _, ok := f.Consts["time"]; ok || len(f.Consts) != 3 {
		t.Errorf("unexpected const groups: %v", f.Consts)
	}
}

func TestParseFileTolerant(t *testing.T) {
	f := ParseFile("package x\n\ntype Order struct {\n\tID uint\n\tOwnerID uint\n}\n\nfunc broken( {\n")
	if fields, ok := (&Index{Types: f.Types}).Struct("Order"); !ok || len(fields) != 2 {
		t.Fatalf("Order not recovered from broken file: %+v", f.Types)
	}
}

func TestMergeSkipsOtherPackages(t *testing.T) {
	a := ParseFile("package app\ntype A struct{ X int }\n")
	b := ParseFile("package app_test\ntype B struct{ Y int }\n")
	idx := Merge("app", []*File{a, b, nil})
	if _, ok := idx.Types["A"]; !ok {
		t.Fatal("A missing")
	}
	if _, ok := idx.Types["B"]; ok {
		t.Fatal("B from app_test merged")
	}
}

func TestElemTypeName(t *testing.T) {
	tests := map[string]string{
		"Order": "Order", "*Order": "Order", "[]Order": "Order", "[]*Order": "Order",
		"[4]Order": "Order", "*[]*Order": "Order", "time.Time": "", "map[string]Order": "",
		"string": "string",
	}
	for in, want := range tests {
		if got := ElemTypeName(in); got != want {
			t.Errorf("ElemTypeName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNilIndex(t *testing.T) {
	var idx *Index
	if _, _, ok := idx.Resolve("X"); ok {
		t.Fatal("nil index resolved")
	}
	if _, ok := idx.Struct("X"); ok {
		t.Fatal("nil index found struct")
	}
}
