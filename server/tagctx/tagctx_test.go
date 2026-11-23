package tagctx

import (
	"reflect"
	"strings"
	"testing"
)

// at runs At on src with the cursor at the single "|" marker.
func at(t *testing.T, src string) (Context, bool) {
	t.Helper()
	i := strings.Index(src, "|")
	if i < 0 || strings.Count(src, "|") != 1 {
		t.Fatalf("source needs exactly one | marker:\n%s", src)
	}
	before := src[:i]
	line := strings.Count(before, "\n")
	col := i - (strings.LastIndex(before, "\n") + 1)
	return At(before+src[i+1:], line, col)
}

// inStruct wraps one field line in a valid file.
func inStruct(field string) string {
	return "package x\n\nimport \"time\"\n\ntype User struct {\n\t" + field + "\n\tName string\n}\n\nvar _ time.Time\n"
}

func TestPositions(t *testing.T) {
	tests := []struct {
		name  string
		field string
		want  Context
	}{
		{"empty closed tag", "ID string `|`",
			Context{Position: Key}},
		{"unclosed tag", "ID string `|",
			Context{Position: Key}},
		{"typing key", "ID string `js|`",
			Context{Position: Key, KeyPartial: "js"}},
		{"typing key unclosed", "ID string `js|",
			Context{Position: Key, KeyPartial: "js"}},
		{"empty value", "ID string `json:\"|\"`",
			Context{Position: Value, Key: "json", ExistingKeys: []string{"json"}}},
		{"value unclosed quote", "ID string `json:\"us|",
			Context{Position: Value, Key: "json", ValuePrefix: "us", ExistingKeys: []string{"json"}}},
		{"option after comma", "ID string `json:\"id,omit|\"`",
			Context{Position: Value, Key: "json", ValuePrefix: "id,omit", ExistingKeys: []string{"json"}}},
		{"escaped quote in value", "ID string `json:\"a\\\"b|\"`",
			Context{Position: Value, Key: "json", ValuePrefix: `a\"b`, ExistingKeys: []string{"json"}}},
		{"second key slot", "ID string `json:\"id\" |`",
			Context{Position: Key, ExistingKeys: []string{"json"}}},
		{"second key value", "ID string `json:\"id\" gorm:\"|\"`",
			Context{Position: Value, Key: "gorm", ExistingKeys: []string{"json", "gorm"}}},
		{"gap between pairs", "ID string `json:\"id\" | gorm:\"x\"`",
			Context{Position: Key, ExistingKeys: []string{"json", "gorm"}}},
		{"retyping existing key excludes itself", "ID string `json| db:\"id\"`",
			Context{Position: Key, KeyPartial: "json", ExistingKeys: []string{"db"}}},
		{"editing key name of complete pair", "ID string `js|on:\"id\"`",
			Context{Position: Key, KeyPartial: "js"}},
		{"after colon", "ID string `json:|`",
			Context{Position: None}},
		{"right before closing backtick", "ID string `json:\"id\"|`",
			Context{Position: Key, ExistingKeys: []string{"json"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := at(t, inStruct(tt.field))
			if !ok {
				t.Fatal("not in tag")
			}
			tt.want.FieldNames = []string{"ID"}
			tt.want.FieldType = "string"
			tt.want.SiblingFields = []string{"Name"}
			tt.want.StructName = "User"
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("\n got  %+v\n want %+v", got, tt.want)
			}
		})
	}
}

func TestFields(t *testing.T) {
	tests := []struct {
		name     string
		field    string
		names    []string
		typ      string
		embedded bool
	}{
		{"grouped", "A, B string `|`", []string{"A", "B"}, "string", false},
		{"unexported", "userID string `|`", []string{"userID"}, "string", false},
		{"pointer selector type", "ExpiresAt *time.Time `|`", []string{"ExpiresAt"}, "*time.Time", false},
		{"slice", "Tags []string `|`", []string{"Tags"}, "[]string", false},
		{"map", "Meta map[string]any `|`", []string{"Meta"}, "map[string]any", false},
		{"embedded", "Base `|`", []string{"Base"}, "Base", true},
		{"embedded pointer qualified", "*time.Location `|`", []string{"Location"}, "*time.Location", true},
		{"embedded generic", "List[int] `|`", []string{"List"}, "List[int]", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := at(t, inStruct(tt.field))
			if !ok {
				t.Fatal("not in tag")
			}
			if !reflect.DeepEqual(got.FieldNames, tt.names) || got.FieldType != tt.typ || got.Embedded != tt.embedded {
				t.Errorf("got names=%v type=%q embedded=%v", got.FieldNames, got.FieldType, got.Embedded)
			}
		})
	}
}

func TestNotATag(t *testing.T) {
	tests := map[string]string{
		"cursor before backtick":                inStruct("ID string| ``"),
		"cursor after tag":                      inStruct("ID string `json:\"id\"` |"),
		"raw string in function":                "package x\n\nfunc f() {\n\ts := `|`\n\t_ = s\n}\n",
		"comment in struct":                     inStruct("// use `|` here"),
		"closing line of multi-line raw string": "package x\n\nvar q = `\nSELECT id\nFROM users |`\n",
	}
	for name, src := range tests {
		t.Run(name, func(t *testing.T) {
			if got, ok := at(t, src); ok {
				t.Errorf("unexpectedly in tag: %+v", got)
			}
		})
	}
}

func TestBrokenFileFallback(t *testing.T) {
	// Syntax error elsewhere in the file: the field line is judged on its own.
	src := "package x\n\ntype User struct {\n\tID string `json:\"|\"`\n}\n\nfunc broken( {\n"
	got, ok := at(t, src)
	if !ok {
		t.Fatal("not in tag")
	}
	if got.Position != Value || got.Key != "json" || !reflect.DeepEqual(got.FieldNames, []string{"ID"}) {
		t.Errorf("got %+v", got)
	}

	// ...and a non-field line in a broken file is still rejected.
	src = "package x\n\nfunc f() {\n\tfmt.Println(`|`)\n\nfunc broken( {\n"
	if got, ok := at(t, src); ok {
		t.Errorf("unexpectedly in tag: %+v", got)
	}
}

func TestCRLF(t *testing.T) {
	src := strings.ReplaceAll(inStruct("ID string `json:\"|\"`"), "\n", "\r\n")
	got, ok := at(t, src)
	if !ok || got.Position != Value || got.Key != "json" {
		t.Fatalf("got %+v ok=%v", got, ok)
	}
}

func TestSiblingFields(t *testing.T) {
	src := "package x\n\ntype Signup struct {\n\tBase\n\tEmail, Backup string\n\tPassword string `validate:\"|\"`\n\tConfirm string\n}\n"
	got, ok := at(t, src)
	if !ok {
		t.Fatal("not in tag")
	}
	want := []string{"Base", "Email", "Backup", "Confirm"}
	if !reflect.DeepEqual(got.SiblingFields, want) {
		t.Fatalf("siblings = %v, want %v", got.SiblingFields, want)
	}
}
