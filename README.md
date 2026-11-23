# Go Tags Autocomplete for Zed

Autocomplete for Go struct tags in [Zed](https://zed.dev): keys, values derived from the field name, options, and option arguments.

```go
type User struct {
	UserID    string    `json:"user_id" gorm:"column:user_id;type:uuid;primaryKey" validate:"required,uuid"`
	CreatedAt time.Time `json:"created_at" gorm:"default:now()"`
}
```

Runs alongside `gopls`; it only answers inside a struct field's backticks.

## What it completes

| Key | Library | Default value style | Options |
|---|---|---|---|
| `json` | encoding/json | `user_id` | `omitempty`, `omitzero`, `string`, `-` |
| `validate` | go-playground/validator | — | rules filtered by field type; `eqfield=` etc. suggest sibling fields |
| `query`, `param`, `form` | echo | `user_id` | — |
| `header` | echo | `User-Id` | — |
| `db` | sqlx | `user_id` | `-` |
| `gorm` | gorm | `column:user_id` | ~30 settings; `type:` suggests Postgres types by field type |
| `env` | sethvargo/go-envconfig | `USER_ID` | `default=`, `required`, `prefix=`, … (inserted as `, `) |
| `yaml`, `toml`, `mapstructure` | — | `user_id` | library options |
| `xml`, `bson` | encoding/xml, mongo-driver | `userId` | library options |

Acronyms count as one word: `UserID` → `user_id` / `userId`, `HTTPServer` → `http_server`.

## Install

In Zed: **zed: extensions** → **Install Dev Extension** → pick the `extension/` folder of this repo.

On first use the extension downloads the language server for your platform from the [latest release](https://github.com/TiveCS/zed-go-tags/releases/latest). No `settings.json` change is needed; Zed starts the server for Go files automatically.

### Using a local build

A `go-tags-ls` on your `PATH` is used instead of the download, which is handy while developing (Go 1.22+):

```sh
cd server && go install .
```

## Releasing

Push a version tag; GitHub Actions tests, builds the server for Linux, macOS and Windows (amd64 + arm64) and publishes the release:

```sh
git tag v0.1.0 && git push origin v0.1.0
```

Bump `version` in `extension/extension.toml` and `extension/Cargo.toml` to match.

## Settings

In Zed's `settings.json`:

```json
{
  "lsp": {
    "go-tags-ls": {
      "initialization_options": {
        "naming": {
          "json": "camelCase",
          "env": "SCREAMING_SNAKE"
        },
        "custom_keys": {
          "mytag": "snake_case"
        }
      }
    }
  }
}
```

- `naming` — change the value style of a built-in key (not `validate`, which has no name).
- `custom_keys` — add your own keys with a style. They appear after the built-in keys.

Styles: `snake_case`, `camelCase`, `PascalCase`, `SCREAMING_SNAKE`, `kebab-case`, `Header-Case`.

Run **editor: restart language server** after changing settings. Invalid entries are skipped and shown as a warning.

## Layout

- `server/` — the language server (Go, no dependencies)
- `extension/` — the Zed extension (Rust → WASM) that starts the server
- `docs/PLAN.md` — build plan · `GLOSSARY.md` — project vocabulary
