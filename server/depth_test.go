package main

import (
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
)

// completeSrc completes a whole source file with the cursor at the last "|".
func completeSrc(t *testing.T, s *server, uri, src string) []completionItem {
	t.Helper()
	i := strings.LastIndex(src, "|")
	before := src[:i]
	s.docs[uri] = before + src[i+1:]
	var p completionParams
	p.TextDocument.URI = uri
	p.Position = position{
		Line:      strings.Count(before, "\n"),
		Character: len([]rune(before[strings.LastIndex(before, "\n")+1:])),
	}
	return s.complete(p)
}

func srcLabels(t *testing.T, src string) []string {
	t.Helper()
	return labels(completeSrc(t, newServer(), testURI, src))
}

func details(items []completionItem) map[string]string {
	out := map[string]string{}
	for _, it := range items {
		out[it.Label] = it.Detail
	}
	return out
}

func assertHas(t *testing.T, got []string, has, hasNot []string) {
	t.Helper()
	for _, h := range has {
		if !slices.Contains(got, h) {
			t.Errorf("missing %q in %v", h, got)
		}
	}
	for _, h := range hasNot {
		if slices.Contains(got, h) {
			t.Errorf("unexpected %q in %v", h, got)
		}
	}
}

func TestNamedTypeResolvesToUnderlying(t *testing.T) {
	src := "package m\n\ntype Status string\n\ntype U struct {\n\tS Status `validate:\"|\"`\n}\n"
	assertHas(t, srcLabels(t, src), []string{"email", "oneof="}, []string{"dive", "gt"})
}

const shop = `package m

type Customer struct {
	ID      uint
	Name    string
	Company Company
	Orders  []Order
	Tags    []string
	At      time.Time
	Price   float64
	Meta    Meta
	X       ext.Thing
}

type Company struct {
	ID   uint
	Name string
}

type Order struct {
	ID      uint
	OwnerID uint
}

type Meta struct{ Raw string }
`

// shopAt puts a tag on one Customer field.
func shopAt(field, tag string) string {
	return strings.Replace(shop, "\t"+field+"\n", "\t"+field+" `"+tag+"`\n", 1)
}

func TestGormRelationFields(t *testing.T) {
	items := completeSrc(t, newServer(), testURI, shopAt("Orders  []Order", `gorm:"foreignKey:|"`))
	got := labels(items)
	want := []string{"ID", "Name", "Company", "Tags", "At", "Price", "Meta", "X", "OwnerID"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %v\nwant %v", got, want)
	}
	d := details(items)
	if d["ID"] != "Customer / Order field" || d["Name"] != "Customer field" || d["OwnerID"] != "Order field" {
		t.Errorf("details = %v", d)
	}

	// belongs-to: same suggestions shape, related struct is Company.
	got = srcLabels(t, shopAt("Company Company", `gorm:"references:|"`))
	assertHas(t, got, []string{"ID", "Name", "Orders"}, []string{"OwnerID"})
}

func TestGormOptionsFilteredByType(t *testing.T) {
	tests := []struct {
		field       string
		has, hasNot []string
	}{
		{"Name    string", []string{"size:", "not null"}, []string{"autoIncrement", "precision:", "foreignKey:", "embedded"}},
		{"ID      uint", []string{"autoIncrement", "autoCreateTime"}, []string{"precision:", "foreignKey:"}},
		{"Price   float64", []string{"precision:", "scale:"}, []string{"autoIncrement", "size:"}},
		{"At      time.Time", []string{"autoCreateTime", "precision:"}, []string{"scale:", "autoIncrement"}},
		{"Orders  []Order", []string{"foreignKey:", "many2many:", "polymorphic:"}, []string{"embedded", "autoIncrement"}},
		{"Company Company", []string{"foreignKey:", "references:", "embedded"}, []string{"many2many:"}},
		{"Tags    []string", []string{"type:"}, []string{"foreignKey:", "many2many:"}},
		{"X       ext.Thing", []string{"foreignKey:", "autoIncrement", "embedded"}, nil}, // unknown: everything
	}
	for _, tt := range tests {
		t.Run(tt.field, func(t *testing.T) {
			assertHas(t, srcLabels(t, shopAt(tt.field, `gorm:"column:x;|"`)), tt.has, tt.hasNot)
		})
	}
}

func TestGormTimeUnits(t *testing.T) {
	if got := srcLabels(t, shopAt("At      time.Time", `gorm:"autoCreateTime:|"`)); len(got) != 0 {
		t.Errorf("time.Time field got %v, want none (milli/nano are for int fields)", got)
	}
	if got := srcLabels(t, shopAt("ID      uint", `gorm:"autoUpdateTime:|"`)); !reflect.DeepEqual(got, []string{"milli", "nano"}) {
		t.Errorf("int field got %v", got)
	}
}

func TestGormPolymorphic(t *testing.T) {
	src := "package m\n\ntype Dog struct {\n\tID uint\n\tToys []Toy `gorm:\"polymorphic:|\"`\n}\n\n" +
		"type Toy struct {\n\tID uint\n\tOwnerID uint\n\tOwnerType string\n\tMakerID uint\n}\n"
	if got := srcLabels(t, src); !reflect.DeepEqual(got, []string{"Owner"}) {
		t.Fatalf("polymorphic: got %v, want [Owner]", got)
	}
	src = strings.Replace(src, "polymorphic:|", "polymorphic:Owner;polymorphicType:|", 1)
	if got := srcLabels(t, src); !reflect.DeepEqual(got, []string{"ID", "OwnerID", "OwnerType", "MakerID"}) {
		t.Fatalf("polymorphicType: got %v", got)
	}
}

func TestGormIndexSubOptions(t *testing.T) {
	tests := []struct {
		tag  string
		want []string
	}{
		{`gorm:"index:|"`, []string{}},         // naming the index: nothing to suggest
		{`gorm:"index:idx_name|"`, []string{}}, // still the name
		{`gorm:"index:idx_name,sort:|"`, []string{"desc", "asc"}},
		{`gorm:"uniqueIndex:u,type:|"`, []string{"btree", "hash", "gin", "gist", "brin"}},
		{`gorm:"index:idx_name,unique,sort:desc,|"`, []string{"type:", "where:", "composite:", "priority:", "expression:", "class:", "length:", "option:", "collate:"}},
	}
	for _, tt := range tests {
		if got := srcLabels(t, shopAt("Name    string", tt.tag)); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%s:\n got  %v\n want %v", tt.tag, got, tt.want)
		}
	}
	got := srcLabels(t, shopAt("Name    string", `gorm:"index:idx_name,|"`))
	assertHas(t, got, []string{"unique", "sort:", "type:", "where:"}, nil)
}

const enums = `package m

type Status string

const (
	StatusActive   Status = "active"
	StatusInactive Status = "inactive"
	StatusBanned   Status = "banned"
)

type Level int

const (
	LevelLow Level = iota + 1
	LevelMid
	LevelHigh
)

type U struct {
	S  Status
	P  *Status
	L  Level
	Ss []Status
}
`

func enumAt(field, tag string) string {
	return strings.Replace(enums, "\t"+field+"\n", "\t"+field+" `"+tag+"`\n", 1)
}

func TestValidateOneofConstants(t *testing.T) {
	tests := []struct {
		field, tag string
		want       []string
	}{
		{"S  Status", `validate:"oneof=|"`, []string{"active inactive banned", "active", "inactive", "banned"}},
		{"S  Status", `validate:"required,oneof=active |"`, []string{"inactive", "banned"}},
		{"P  *Status", `validate:"oneof=|"`, []string{"active inactive banned", "active", "inactive", "banned"}},
		{"L  Level", `validate:"oneof=|"`, []string{"1 2 3", "1", "2", "3"}},
		{"Ss []Status", `validate:"dive,oneof=|"`, []string{"active inactive banned", "active", "inactive", "banned"}},
	}
	for _, tt := range tests {
		if got := srcLabels(t, enumAt(tt.field, tt.tag)); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%s %s:\n got  %v\n want %v", tt.field, tt.tag, got, tt.want)
		}
	}
}

func TestEnvStructPrefix(t *testing.T) {
	src := "package m\n\ntype DatabaseConfig struct{ Host string }\n\ntype Config struct {\n\tDB DatabaseConfig `env:\"|\"`\n\tPort int\n}\n"
	if got := srcLabels(t, src); !reflect.DeepEqual(got, []string{", prefix=DB_"}) {
		t.Fatalf("value: got %v", got)
	}
	keySrc := strings.Replace(src, "`env:\"|\"`", "`|`", 1)
	it := find(t, completeSrc(t, newServer(), testURI, keySrc), "env")
	if it.TextEdit.NewText != `env:", prefix=${1:DB_}"` {
		t.Fatalf("key inserts %s", it.TextEdit.NewText)
	}
	// non-struct field keeps the variable name.
	portSrc := strings.Replace(src, "\tPort int\n", "\tPort int `env:\"|\"`\n", 1)
	portSrc = strings.Replace(portSrc, "`env:\"|\"`\n\tPort", "`env:\"\"`\n\tPort", 1)
	if got := srcLabels(t, portSrc); !reflect.DeepEqual(got, []string{"PORT"}) {
		t.Fatalf("non-struct: got %v", got)
	}
}

func TestMapstructureOnStruct(t *testing.T) {
	src := "package m\n\ntype Base struct{ ID int }\n\ntype U struct {\n\tBase `mapstructure:\",|\"`\n}\n"
	if got := srcLabels(t, src); !reflect.DeepEqual(got, []string{"squash", "omitempty"}) {
		t.Fatalf("got %v", got)
	}
}

// --- package index across files ---

func fileURI(p string) string {
	p = filepath.ToSlash(p)
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return (&url.URL{Scheme: "file", Path: p}).String()
}

func TestPackageAcrossFiles(t *testing.T) {
	dir := t.TempDir()
	write := func(name, src string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	customer := "package shop\n\ntype Customer struct {\n\tID uint\n\tOrders []Order `gorm:\"foreignKey:|\"`\n}\n"
	write("customer.go", strings.Replace(customer, "|", "", 1))
	orderPath := write("order.go", "package shop\n\ntype Order struct {\n\tOwnerID uint\n}\n")
	write("other_test.go", "package shop_test\n\ntype Order struct {\n\tWrong uint\n}\n")
	uri := fileURI(filepath.Join(dir, "customer.go"))

	s := newServer()
	got := labels(completeSrc(t, s, uri, customer))
	assertHas(t, got, []string{"ID", "OwnerID"}, []string{"Wrong"})

	// Disk change is picked up (new mtime and size).
	write("order.go", "package shop\n\ntype Order struct {\n\tOwnerID uint\n\tPlacedBy uint\n}\n")
	future := time.Now().Add(time.Hour)
	os.Chtimes(orderPath, future, future)
	assertHas(t, labels(completeSrc(t, s, uri, customer)), []string{"PlacedBy"}, nil)

	// An open, unsaved buffer wins over the file on disk.
	s.docs[fileURI(orderPath)] = "package shop\n\ntype Order struct {\n\tLiveField uint\n}\n"
	assertHas(t, labels(completeSrc(t, s, uri, customer)), []string{"LiveField"}, []string{"PlacedBy"})
	delete(s.docs, fileURI(orderPath))

	// type_search "file": other files are not read.
	s.typeSearch = typeSearchFile
	assertHas(t, labels(completeSrc(t, s, uri, customer)), []string{"ID"}, []string{"OwnerID", "PlacedBy"})
}

func TestTypeSearchSetting(t *testing.T) {
	s := newServer()
	s.configure([]byte(`{"type_search":"file"}`))
	if s.typeSearch != typeSearchFile || len(s.problems) != 0 {
		t.Fatalf("typeSearch=%s problems=%v", s.typeSearch, s.problems)
	}
	s = newServer()
	s.configure([]byte(`{"type_search":"workspace"}`))
	if s.typeSearch != typeSearchPackage || len(s.problems) != 1 {
		t.Fatalf("bad value: typeSearch=%s problems=%v", s.typeSearch, s.problems)
	}
}

func TestURIToPath(t *testing.T) {
	p, ok := uriToPath(fileURI(filepath.Join(t.TempDir(), "a b.go")))
	if !ok || filepath.Base(p) != "a b.go" {
		t.Fatalf("got %q %v", p, ok)
	}
	if _, ok := uriToPath("untitled:Untitled-1"); ok {
		t.Fatal("non-file URI accepted")
	}
}
