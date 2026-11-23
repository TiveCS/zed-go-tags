# Build Plan — Go Tags Autocomplete for Zed

Status: **Phase 0 done** (verified in Zed 2026-11-23). **Phase 1 done** (verified in Zed). **Phase 2 done** (verified in Zed). **Phase 3 done** (verified in Zed). **Phase 3b done** (verified in Zed). **Phase 4 done** in tests; Zed check pending. Vocabulary: see [GLOSSARY.md](../GLOSSARY.md).

## Repo layout

```
/
├── GLOSSARY.md
├── docs/PLAN.md
├── extension/                 # Zed extension (Rust → WASM)
│   ├── extension.toml         # id, name, language_servers.go-tags { languages = ["Go"] }
│   ├── Cargo.toml             # zed_extension_api
│   └── src/lib.rs             # locate/download server binary, return Command
└── server/                    # Language server (Go)
    ├── go.mod
    ├── main.go                # stdio LSP loop
    ├── lsp/                   # initialize, didOpen/didChange, completion
    ├── tagctx/                # cursor → TagContext (field, type, key, value, option position)
    ├── naming/                # field name → Naming Style (camel/snake/SCREAMING/Header-Case, acronym lowering)
    ├── catalog/               # per-key definitions: order, default style, value template, options
    │   ├── json.go, gorm.go, validate.go, env.go, ...
    └── testdata/              # Go sources with cursor markers for table tests
```

## Phases

### Phase 0 — Skeleton (tracer bullet)
- Go server answering `initialize` + `textDocument/completion` with a hardcoded `json` item.
- Zed extension launching the server binary from `PATH`.
- Load as dev extension in Zed (`zed: install dev extension`), confirm item appears inside backticks next to gopls.
- **Done when:** typing `` ` `` on a struct field in Zed shows `json`.

### Phase 1 — Tag context detection
- Parse the buffer with `go/parser` (tolerant of broken code; fall back to line-level scan when the file doesn't parse mid-edit).
- Determine: inside a Struct Tag? which field name(s) + Go type? cursor at key position / value position / option position? which keys already present?
- **Done when:** table tests cover each cursor position, grouped fields (`A, B string` → first name), unexported fields, embedded fields, unparseable files.

### Phase 2 — Tag Keys + Tag Values
- Naming styles with acronym lowering (`UserID` → `userId`, `HTTPServer` → `httpServer`, `User-Id`, `USER_ID`, `user_id`).
- Default Naming Style per key (overridable in settings):

  | Style | Keys |
  |---|---|
  | snake_case (`user_id`) | `json`, `yaml`, `db`, `toml`, `mapstructure`, `form`, `query`, `param`, `gorm` (`column:user_id`) |
  | camelCase (`userId`) | `xml`, `bson` |
  | Http-Header-Case (`User-Id`) | `header` |
  | SCREAMING_SNAKE (`USER_ID`) | `env` |
  | none (rules only) | `validate` |

- Key catalog in popularity order: `json, validate, query, param, form, header, db, gorm, env, yaml, xml, toml, mapstructure, bson`; hide keys already used.
- Selecting a key inserts `key:"value"` as a snippet with the value selected; `gorm` → `column:<snake>`; `validate` → `validate:"` and re-trigger completion.
- **Done when:** unit tests for every key's insertion for a set of field names.

### Phase 3 — Tag Options
- `json`/`yaml`/`xml`/`toml`/`bson`/`mapstructure`: options after `,` (no space).
- `gorm`: options after `;`.
- `env` (sethvargo/go-envconfig): options after `, ` (with space).
- `validate` (go-playground/validator): rules filtered by Go type (string / numeric / slice-map / any).
- **Done when:** tests per key for option suggestions, including type filtering.

### Phase 3b — Option Arguments
Suggest the text after `:` / `=` inside a Tag Option. Postgres only for now (MySQL switch: later list).
- `gorm` `type:` filtered by Go type:
  - `string` → `uuid`, `text`, `varchar(255)`, `char(36)`, `citext`
  - ints → `bigint`, `integer`, `smallint`, `bigserial`, `serial`
  - floats → `numeric(10,2)`, `double precision`, `real`
  - `time.Time` → `timestamptz`, `timestamp`, `date`, `time`
  - `bool` → `boolean`
  - `[]byte` / named / other → all of the above plus `jsonb`, `json`, `bytea`, `inet`, `text[]`
- `gorm` `serializer:` → `json`, `gob`, `unixtime`
- `gorm` `default:` → `gen_random_uuid()`, `now()`, `CURRENT_TIMESTAMP`, `true`, `false`, `0`, `''`
- `gorm` `<-:` → `create`, `update`, `false`; `->:` → `false`; `-:` → `migration`, `all`
- `gorm` `constraint:` → `OnUpdate:CASCADE,OnDelete:SET NULL`, `OnUpdate:CASCADE,OnDelete:CASCADE`, `OnDelete:SET NULL`, `OnDelete:CASCADE`
- `validate` `datetime=` → `2006-01-02`, `2006-01-02T15:04:05Z07:00`, `15:04`
- `validate` field-reference rules (`eqfield=`, `nefield=`, `gtfield=`…, `required_if=`, `required_with=`…, `excluded_*=`) → names of the other fields in the same struct (needs the whole struct from the AST, not just the field line).
- **Done when:** tests per argument list, incl. Go-type filtering and sibling field names.

### Phase 4 — User settings
- Read `initialization_options` from Zed `settings.json` (`lsp.go-tags.initialization_options`): per-key Naming Style override + custom keys.
- Document example config in README.
- **Done when:** override + custom key verified in Zed.

### Phase 5 — Distribution
- GitHub Actions: cross-compile server (windows/linux/darwin × amd64/arm64) and attach to Releases.
- Extension: use `PATH` binary if present, else download matching release asset.
- README + MIT LICENSE. Public registry PR deferred.

## Facts verified in Phase 0
- `zed_extension_api` 0.7.0; `language_server_command(&mut self, &LanguageServerId, &Worktree) -> Result<Command>`.
- Zed enables the extension's Go language server alongside gopls automatically — no `settings.json` change needed.
- Zed honors server-declared trigger characters and snippet insertion (`json:"$1"` places cursor between quotes).

## Later list (not v1)
Ctrl+. code actions (add tags to all fields), hover docs for options, diagnostics (malformed tag, duplicate json names), `msgpack`.
