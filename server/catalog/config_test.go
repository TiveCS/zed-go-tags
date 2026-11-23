package catalog

import (
	"strings"
	"testing"

	"go-tags-ls/naming"
)

func TestConfigureDefaults(t *testing.T) {
	keys, problems := Configure(Config{})
	if len(problems) != 0 || len(keys) != len(Keys) {
		t.Fatalf("keys=%d problems=%v", len(keys), problems)
	}
}

func TestConfigureNamingOverride(t *testing.T) {
	keys, problems := Configure(Config{Naming: map[string]naming.Style{"json": naming.Camel, "gorm": naming.Camel}})
	if len(problems) != 0 {
		t.Fatalf("problems: %v", problems)
	}
	json, _ := Find(keys, "json")
	gorm, _ := Find(keys, "gorm")
	if json.Value("UserID") != "userId" || gorm.Value("UserID") != "column:userId" {
		t.Fatalf("json=%s gorm=%s", json.Value("UserID"), gorm.Value("UserID"))
	}
	// The built-in table must stay untouched.
	if orig, _ := Find(Keys, "json"); orig.Style != naming.Snake {
		t.Fatal("Configure mutated the built-in Keys")
	}
}

func TestConfigureCustomKeys(t *testing.T) {
	keys, problems := Configure(Config{CustomKeys: map[string]naming.Style{
		"zeta":  naming.Kebab,
		"alpha": naming.Snake,
	}})
	if len(problems) != 0 {
		t.Fatalf("problems: %v", problems)
	}
	n := len(keys)
	if n != len(Keys)+2 || keys[n-2].Name != "alpha" || keys[n-1].Name != "zeta" {
		t.Fatalf("custom keys not appended alphabetically: %v %v", keys[n-2].Name, keys[n-1].Name)
	}
	if keys[n-1].Value("UserID") != "user-id" {
		t.Fatalf("zeta value = %s", keys[n-1].Value("UserID"))
	}
}

func TestConfigureProblems(t *testing.T) {
	keys, problems := Configure(Config{
		Naming: map[string]naming.Style{
			"nope":     naming.Snake,       // unknown key
			"validate": naming.Snake,       // no name
			"json":     naming.Style("XX"), // bad style
		},
		CustomKeys: map[string]naming.Style{
			"db":     naming.Snake,       // already built in
			"my tag": naming.Snake,       // invalid name
			"ok":     naming.Style("YY"), // bad style
		},
	})
	if len(problems) != 6 {
		t.Fatalf("want 6 problems, got %d: %v", len(problems), problems)
	}
	for _, want := range []string{"naming.nope", "naming.validate", "naming.json", "custom_keys.db", `custom_keys."my tag"`, "custom_keys.ok"} {
		found := false
		for _, p := range problems {
			found = found || strings.HasPrefix(p, want)
		}
		if !found {
			t.Errorf("no problem reported for %s: %v", want, problems)
		}
	}
	// Bad settings leave the defaults in place.
	if json, _ := Find(keys, "json"); json.Style != naming.Snake || len(keys) != len(Keys) {
		t.Fatal("bad settings changed keys")
	}
}
