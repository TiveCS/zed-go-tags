package main

import (
	"reflect"
	"slices"
	"strings"
	"testing"
)

// completeAt runs completion on a struct with one field line, cursor at "|".
func completeAt(t *testing.T, field string) []completionItem {
	t.Helper()
	src := "package x\n\ntype User struct {\n\t" + field + "\n}\n"
	i := strings.LastIndex(src, "|") // last: validate uses | as OR separator
	before := src[:i]
	line := strings.Count(before, "\n")
	char := len([]rune(before[strings.LastIndex(before, "\n")+1:])) // ASCII-only fixtures
	s := newServer()
	s.docs[testURI] = before + src[i+1:]
	var p completionParams
	p.TextDocument.URI = testURI
	p.Position = position{Line: line, Character: char}
	return s.complete(p)
}

func labels(items []completionItem) []string {
	out := []string{}
	for _, it := range items {
		out = append(out, it.Label)
	}
	return out
}

func find(t *testing.T, items []completionItem, label string) completionItem {
	t.Helper()
	for _, it := range items {
		if it.Label == label {
			return it
		}
	}
	t.Fatalf("no item %q in %v", label, labels(items))
	return completionItem{}
}

func TestKeyOrder(t *testing.T) {
	got := labels(completeAt(t, "UserID string `|`"))
	want := []string{"json", "validate", "query", "param", "form", "header", "db",
		"gorm", "env", "yaml", "xml", "toml", "mapstructure", "bson"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("keys = %v\nwant   %v", got, want)
	}
}

func TestKeyInsertsPair(t *testing.T) {
	items := completeAt(t, "UserID string `|`")
	want := map[string]string{
		"json":         `json:"${1:user_id}"`,
		"validate":     `validate:"$1"`,
		"query":        `query:"${1:user_id}"`,
		"param":        `param:"${1:user_id}"`,
		"form":         `form:"${1:user_id}"`,
		"header":       `header:"${1:User-Id}"`,
		"db":           `db:"${1:user_id}"`,
		"gorm":         `gorm:"${1:column:user_id}"`,
		"env":          `env:"${1:USER_ID}"`,
		"yaml":         `yaml:"${1:user_id}"`,
		"xml":          `xml:"${1:userId}"`,
		"toml":         `toml:"${1:user_id}"`,
		"mapstructure": `mapstructure:"${1:user_id}"`,
		"bson":         `bson:"${1:userId}"`,
	}
	for key, snippet := range want {
		it := find(t, items, key)
		if it.TextEdit == nil || it.TextEdit.NewText != snippet {
			t.Errorf("%s inserts %+v, want %s", key, it.TextEdit, snippet)
		}
		if it.InsertTextFormat != insertTextFormatSnippet {
			t.Errorf("%s not a snippet", key)
		}
	}
}

func TestKeyHidesExisting(t *testing.T) {
	got := labels(completeAt(t, "ID string `json:\"id\" gorm:\"primaryKey\" |`"))
	for _, k := range got {
		if k == "json" || k == "gorm" {
			t.Fatalf("existing key %s offered: %v", k, got)
		}
	}
	if len(got) != 12 {
		t.Fatalf("got %d keys, want 12: %v", len(got), got)
	}
}

func TestKeyReplacesPartial(t *testing.T) {
	// "\tID string `js|" : backtick at char 11, cursor at 14, typed "js" -> replace 12..14.
	it := find(t, completeAt(t, "ID string `js|`"), "json")
	r := it.TextEdit.Range
	if r.Start.Character != 12 || r.End.Character != 14 {
		t.Fatalf("range = %+v, want 12..14", r)
	}
}

func TestGroupedFieldUsesFirstName(t *testing.T) {
	it := find(t, completeAt(t, "FirstName, LastName string `|`"), "json")
	if it.TextEdit.NewText != `json:"${1:first_name}"` {
		t.Fatalf("got %s", it.TextEdit.NewText)
	}
}

func TestEmbeddedField(t *testing.T) {
	it := find(t, completeAt(t, "*BaseModel `|`"), "json")
	if it.TextEdit.NewText != `json:"${1:base_model}"` {
		t.Fatalf("got %s", it.TextEdit.NewText)
	}
}

func TestNameSuggestions(t *testing.T) {
	tests := []struct {
		name, field string
		want        []string // leading labels
		typed       int      // chars replaced before cursor
	}{
		{"json empty", "UserID string `json:\"|\"`", []string{"user_id", "-"}, 0},
		{"json partial", "UserID string `json:\"us|\"`", []string{"user_id", "-"}, 2},
		{"header has no skip", "RequestID string `header:\"|\"`", []string{"Request-Id"}, 0},
		{"env", "DatabaseURL string `env:\"|\"`", []string{"DATABASE_URL"}, 0},
		{"xml camel", "UserID string `xml:\"|\"`", []string{"userId", "-"}, 0},
		{"gorm first part", "UserID string `gorm:\"|\"`", []string{"column:user_id", "primaryKey"}, 0},
		{"gorm after other setting", "UserID string `gorm:\"primaryKey; co|\"`", []string{"column:user_id", "type:"}, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			items := completeAt(t, tt.field)
			got := labels(items)
			if len(got) < len(tt.want) || !reflect.DeepEqual(got[:len(tt.want)], tt.want) {
				t.Fatalf("got %v, want prefix %v", got, tt.want)
			}
			if tt.want[0] != "-" && len(got) > len(tt.want) && got[len(tt.want)] == "omitempty" {
				t.Fatalf("options offered in name part: %v", got)
			}
			r := items[0].TextEdit.Range
			if r.End.Character-r.Start.Character != tt.typed {
				t.Errorf("replaces %d chars, want %d", r.End.Character-r.Start.Character, tt.typed)
			}
		})
	}
}

func TestNoValueSuggestions(t *testing.T) {
	for name, field := range map[string]string{
		"unknown key":      "ID string `custom:\"|\"`",
		"after colon":      "ID string `json:|`",
		"echo key options": "ID string `query:\"id,|\"`",
		"sqlx db options":  "ID string `db:\"id,|\"`",
	} {
		if got := labels(completeAt(t, field)); len(got) != 0 {
			t.Errorf("%s: got %v, want none", name, got)
		}
	}
}

func TestOptions(t *testing.T) {
	tests := []struct {
		name, field string
		want        []string // exact labels
	}{
		{"json", "ID string `json:\"id,|\"`", []string{"omitempty", "omitzero", "string"}},
		{"json hides used", "ID string `json:\"id,omitempty,|\"`", []string{"omitzero", "string"}},
		{"yaml on string", "ID string `yaml:\"id,|\"`", []string{"omitempty"}},
		{"yaml on slice", "Tags []string `yaml:\"tags,|\"`", []string{"omitempty", "flow"}},
		{"yaml on map", "Meta map[string]string `yaml:\"meta,|\"`", []string{"omitempty", "flow", "inline"}},
		{"mapstructure unknown type", "Base Base `mapstructure:\",|\"`", []string{"squash", "remain", "omitempty"}},
		{"bson on string", "ID string `bson:\"id,|\"`", []string{"omitempty", "omitzero"}},
		{"bson on int64", "Count int64 `bson:\"count,|\"`", []string{"omitempty", "omitzero", "minsize", "truncate"}},
		{"json string option not on slice", "Tags []string `json:\"tags,|\"`", []string{"omitempty", "omitzero"}},
		{"env", "Port string `env:\"PORT,|\"`", []string{"default=", "required", "prefix=", "overwrite", "delimiter=", "separator=", "noinit", "decodeunset"}},
		{"env hides used", "Port string `env:\"PORT, required, |\"`", []string{"default=", "prefix=", "overwrite", "delimiter=", "separator=", "noinit", "decodeunset"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := labels(completeAt(t, tt.field)); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got  %v\nwant %v", got, tt.want)
			}
		})
	}
}

func TestOptionInsertion(t *testing.T) {
	tests := []struct {
		name, field, label, newText string
		format, typed               int
	}{
		{"plain option", "ID string `json:\"id,om|\"`", "omitempty", "omitempty", insertTextFormatPlain, 2},
		{"arg option is snippet", "Age int `validate:\"m|\"`", "min=", "min=$1", insertTextFormatSnippet, 1},
		{"gorm arg", "Name string `gorm:\"column:name;|\"`", "size:", "size:$1", insertTextFormatSnippet, 0},
		{"env adds space after bare comma", "Port string `env:\"PORT,|\"`", "required", " required", insertTextFormatPlain, 0},
		{"env no extra space", "Port string `env:\"PORT, |\"`", "required", "required", insertTextFormatPlain, 0},
		{"env space before arg", "Port string `env:\"PORT,|\"`", "default=", " default=$1", insertTextFormatSnippet, 0},
		{"env typed after space", "Port string `env:\"PORT, req|\"`", "required", "required", insertTextFormatPlain, 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			it := find(t, completeAt(t, tt.field), tt.label)
			if it.TextEdit.NewText != tt.newText || it.InsertTextFormat != tt.format {
				t.Errorf("inserts %q (format %d), want %q (format %d)", it.TextEdit.NewText, it.InsertTextFormat, tt.newText, tt.format)
			}
			r := it.TextEdit.Range
			if r.End.Character-r.Start.Character != tt.typed {
				t.Errorf("replaces %d chars, want %d", r.End.Character-r.Start.Character, tt.typed)
			}
		})
	}
}

func TestGormOptions(t *testing.T) {
	got := labels(completeAt(t, "ID uint `gorm:\"column:id;PRIMARYKEY;|\"`"))
	if slices.Contains(got, "column:id") || slices.Contains(got, "primaryKey") {
		t.Fatalf("used settings offered (case-insensitive): %v", got)
	}
	if !slices.Contains(got, "not null") || !slices.Contains(got, "autoIncrement") {
		t.Fatalf("missing settings: %v", got)
	}
}

func TestValidateFilteredByType(t *testing.T) {
	tests := []struct {
		field       string
		has, hasNot []string
	}{
		{"Email string `validate:\"|\"`",
			[]string{"required", "email", "min=", "oneof=", "uuid"},
			[]string{"dive", "keys", "gt"}},
		{"Age int `validate:\"|\"`",
			[]string{"required", "min=", "gte=", "oneof="},
			[]string{"email", "dive", "uuid"}},
		{"Score *float64 `validate:\"|\"`",
			[]string{"min=", "lte="},
			[]string{"email"}},
		{"Tags []string `validate:\"|\"`",
			[]string{"dive", "unique", "min="},
			[]string{"email", "keys", "oneof="}},
		{"Meta map[string]string `validate:\"|\"`",
			[]string{"dive", "keys", "endkeys"},
			[]string{"email"}},
		{"Start time.Time `validate:\"|\"`",
			[]string{"required", "gt", "lte", "gtfield="},
			[]string{"min=", "gt=", "email"}},
		{"Active bool `validate:\"|\"`",
			[]string{"required", "eq="},
			[]string{"min=", "email"}},
		{"Status Status `validate:\"|\"`", // named type: unknown, show everything
			[]string{"email", "min=", "dive", "gt"},
			nil},
	}
	for _, tt := range tests {
		got := labels(completeAt(t, tt.field))
		for _, h := range tt.has {
			if !slices.Contains(got, h) {
				t.Errorf("%s: missing %s", tt.field, h)
			}
		}
		for _, h := range tt.hasNot {
			if slices.Contains(got, h) {
				t.Errorf("%s: unexpected %s", tt.field, h)
			}
		}
	}
}

func TestValidateSeparatorsAndDive(t *testing.T) {
	// "|" is validator's OR separator; used rules are hidden after it too.
	got := labels(completeAt(t, "Code string `validate:\"required,hexcolor|rg|\"`"))
	if slices.Contains(got, "required") || slices.Contains(got, "hexcolor") || !slices.Contains(got, "rgb") {
		t.Fatalf("used rules offered: %v", got)
	}
	// After dive, element rules may repeat ones used before it.
	got = labels(completeAt(t, "Tags []string `validate:\"required,min=1,dive,|\"`"))
	if !slices.Contains(got, "required") || !slices.Contains(got, "min=") {
		t.Fatalf("rules after dive should allow repeats: %v", got)
	}
}

func TestGormTypeArgs(t *testing.T) {
	tests := []struct {
		field       string
		has, hasNot []string
	}{
		{"ID string `gorm:\"type:|\"`", []string{"uuid", "text", "varchar(255)", "citext"}, []string{"bigint", "jsonb", "timestamptz"}},
		{"ID *int64 `gorm:\"type:|\"`", []string{"bigint", "bigserial"}, []string{"uuid", "numeric(10,2)"}},
		{"Price float64 `gorm:\"type:|\"`", []string{"numeric(10,2)", "double precision"}, []string{"bigint"}},
		{"At time.Time `gorm:\"type:|\"`", []string{"timestamptz", "date"}, []string{"uuid"}},
		{"On bool `gorm:\"type:|\"`", []string{"boolean"}, []string{"text"}},
		{"Raw []byte `gorm:\"type:|\"`", []string{"bytea", "jsonb", "uuid", "bigint"}, nil},
		{"Meta Meta `gorm:\"type:|\"`", []string{"jsonb", "text"}, nil},
	}
	for _, tt := range tests {
		got := labels(completeAt(t, tt.field))
		for _, h := range tt.has {
			if !slices.Contains(got, h) {
				t.Errorf("%s: missing %s in %v", tt.field, h, got)
			}
		}
		for _, h := range tt.hasNot {
			if slices.Contains(got, h) {
				t.Errorf("%s: unexpected %s", tt.field, h)
			}
		}
	}
}

func TestArgReplacesTypedArgument(t *testing.T) {
	items := completeAt(t, "ID string `gorm:\"primaryKey;type:uu|\"`")
	it := find(t, items, "uuid")
	r := it.TextEdit.Range
	if it.TextEdit.NewText != "uuid" || r.End.Character-r.Start.Character != 2 {
		t.Fatalf("inserts %q over %d chars", it.TextEdit.NewText, r.End.Character-r.Start.Character)
	}
	// While typing an argument, the option list must not show.
	if slices.Contains(labels(items), "not null") {
		t.Fatal("options shown while typing an argument")
	}
}

func TestOtherGormArgs(t *testing.T) {
	tests := []struct {
		field string
		want  []string
	}{
		{"ID string `gorm:\"serializer:|\"`", []string{"json", "gob", "unixtime"}},
		{"ID string `gorm:\"<-:|\"`", []string{"create", "update", "false"}},
		{"ID string `gorm:\"->:|\"`", []string{"false"}},
		{"ID string `gorm:\"-:|\"`", []string{"migration", "all"}},
		{"ID string `gorm:\"default:|\"`", []string{"gen_random_uuid()", "''"}},
		{"At time.Time `gorm:\"default:|\"`", []string{"now()", "CURRENT_TIMESTAMP"}},
		{"On bool `gorm:\"DEFAULT:|\"`", []string{"true", "false"}}, // gorm is case-insensitive
		{"UserID uint `gorm:\"constraint:|\"`", []string{"OnUpdate:CASCADE,OnDelete:SET NULL", "OnUpdate:CASCADE,OnDelete:CASCADE", "OnDelete:SET NULL", "OnDelete:CASCADE"}},
		{"ID string `gorm:\"size:|\"`", []string{}}, // takes an argument, nothing to suggest
	}
	for _, tt := range tests {
		if got := labels(completeAt(t, tt.field)); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%s:\n got  %v\n want %v", tt.field, got, tt.want)
		}
	}
}

func TestGormBareOptionUsedWithArg(t *testing.T) {
	got := labels(completeAt(t, "ID string `gorm:\"<-:create;|\"`"))
	if slices.Contains(got, "<-") {
		t.Fatalf("<- offered again: %v", got)
	}
}

func TestValidateDatetimeArgs(t *testing.T) {
	got := labels(completeAt(t, "Day string `validate:\"datetime=|\"`"))
	want := []string{"2006-01-02", "2006-01-02T15:04:05Z07:00", "15:04"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v", got)
	}
}

// signupAt places field lines inside a struct with sibling fields.
func signupAt(t *testing.T, tag string) []string {
	t.Helper()
	return labels(completeAt(t, "Password string\n\tConfirm string `validate:\""+tag+"\"`\n\tEmail string\n\tPhone string\n\tRole string"))
}

func TestValidateFieldRefs(t *testing.T) {
	all := []string{"Password", "Email", "Phone", "Role"}
	tests := []struct {
		tag  string
		want []string
	}{
		{"eqfield=|", all},
		{"eqfield=Pa|", all},        // Zed filters by typed text
		{"required,nefield=|", all}, // after other rules
		{"eqfield=Password |", []string{}},
		{"required_with=|", all},
		{"required_with=Email |", []string{"Password", "Phone", "Role"}}, // listed fields hidden
		{"required_if=|", all},
		{"required_if=Role |", []string{}}, // value slot
		{"required_if=Role admin |", all},  // next field slot
		{"excluded_unless=Role admin Email |", []string{}},
	}
	for _, tt := range tests {
		if got := signupAt(t, tt.tag); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%s:\n got  %v\n want %v", tt.tag, got, tt.want)
		}
	}
}

func TestFieldRefReplacesLastToken(t *testing.T) {
	items := completeAt(t, "Password string\n\tConfirm string `validate:\"required_with=Password Em|\"`\n\tEmail string")
	it := find(t, items, "Email")
	r := it.TextEdit.Range
	if r.End.Character-r.Start.Character != 2 {
		t.Fatalf("replaces %d chars, want 2", r.End.Character-r.Start.Character)
	}
}
