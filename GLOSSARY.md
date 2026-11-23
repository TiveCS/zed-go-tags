# Go Tags Autocomplete

A Zed extension that suggests completions while writing struct tags on Go struct fields.

## Language

### Tag anatomy

**Struct Tag**:
The backtick-quoted string after a Go struct field, e.g. `` `json:"id,omitempty" db:"user_id"` ``.
_Avoid_: Annotation, field tag, label

**Tag Key**:
The name before the colon in a Struct Tag, identifying which library reads it, e.g. `json`, `db`.
_Avoid_: Tag name, tag type

**Tag Value**:
The quoted name after a Tag Key, usually derived from the field name, e.g. `user_id`.
_Avoid_: Tag name, alias

**Tag Option**:
A comma-separated modifier following the Tag Value, e.g. `omitempty`, `-`, `string`.
_Avoid_: Flag, modifier

**Option Argument**:
The text after `=` or `:` inside a Tag Option, e.g. `uuid` in `type:uuid`, `5` in `min=5`.
_Avoid_: Option value, parameter

### Behaviour

**Naming Style**:
The casing rule used to turn a field name into a suggested Tag Value, e.g. camelCase, snake_case, SCREAMING_SNAKE.
_Avoid_: Case, format, convention
