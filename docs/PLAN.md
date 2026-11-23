# Build Plan — Go Tags Autocomplete for Zed

Status: **Phase 0 done** (verified in Zed 2026-11-23). **Phase 1 done** (verified in Zed). **Phase 2 done** (verified in Zed). **Phase 3 done** (verified in Zed). **Phase 3b done** (verified in Zed). **Phase 4 done** (verified in Zed). **Phase 5 done** (local commit; not published). **Phase 6 done** in tests; Zed check pending. Vocabulary: see [GLOSSARY.md](../GLOSSARY.md).

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

### Phase 6 — Depth (ships in v0.1.0, before publishing)
Foundation: a **package index** — parse every `.go` file in the current file's folder (open files from Zed's live copy, the rest from disk; cached by modification time) into: struct types → their fields, named types → their underlying type, typed constants → their values.
- Setting `type_search`: `"package"` (default) or `"file"` (only the current file).
- Named types resolve through the index: `type Status string` now classifies as String (better validate filtering); a struct type classifies as **Struct** (new class).

Features:
- **G1 gorm relations** — `foreignKey:`, `references:`, `joinForeignKey:`, `joinReferences:` suggest fields of this struct *and* of the related struct (from the field's type: `Order`, `*Order`, `[]Order`, `[]*Order`), each labelled with its struct. Belongs-to vs has-one/has-many can't be told apart from syntax, so both sides are offered.
- **G2 gorm index sub-options** — after `index:name` / `uniqueIndex:name`, a `,` offers `unique`, `sort:desc`, `sort:asc`, `type:btree`, `type:hash`, `type:gin`, `type:gist`, `where:`, `composite:`, `priority:`, `expression:`, `class:`, `length:`, `option:`.
- **G5 gorm arguments** — `autoCreateTime:` / `autoUpdateTime:` → `milli`, `nano`; `polymorphic:` → struct names in the package; new settings `polymorphicType:`, `polymorphicId:`, `polymorphicValue:`, `autoIncrementIncrement:`.
- **G6 gorm settings filtered by type** — e.g. `autoIncrement` ints only, `precision:`/`scale:` floats, `autoCreateTime` time/ints, `size:` strings/ints, relation settings only on struct/slice-of-struct fields.
- **V1 validate `oneof=` from constants** — for a field of named type `T`, suggest the typed constants of `T` in the package: one item with all values space-separated, plus each value. Supports explicit string/number literals and simple `iota` / `iota + n` sequences.
- **O1 options filtered by type** — `json:",string"` numbers/bools/strings; `yaml:",flow"` slices/maps/structs; `yaml:",inline"` / `bson:",inline"` structs/maps; `bson:",minsize"` int64; `bson:",truncate"` ints; `mapstructure:",squash"` structs; `,remain` maps; `xml:",attr"`/`chardata`/`cdata` non-struct.
- **O2 env on struct fields** — for a struct-typed field, the env value suggestion is `, prefix=DATABASE_` (go-envconfig nested struct) instead of a variable name.

**Done when:** package index tests (multi-file, live-buffer override, cache refresh), tests per feature, README documents `type_search`.

## Facts verified in Phase 0
- `zed_extension_api` 0.7.0; `language_server_command(&mut self, &LanguageServerId, &Worktree) -> Result<Command>`.
- Zed enables the extension's Go language server alongside gopls automatically — no `settings.json` change needed.
- Zed honors server-declared trigger characters and snippet insertion (`json:"$1"` places cursor between quotes).

## Later list (not v1)
Ctrl+. code actions (add tags to all fields), hover docs for options, diagnostics (malformed tag, duplicate json names), `msgpack`.
