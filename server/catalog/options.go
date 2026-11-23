package catalog

// Option is a Tag Option. A Name ending in '=' or ':' takes an argument
// (e.g. "min=", "size:"); the cursor lands after it on insert.
type Option struct {
	Name   string
	Detail string
	For    Class // field types it applies to; 0 means any (validate only)

	// Args are suggested Option Arguments. Options without a trailing
	// '=' / ':' (gorm "<-", "->", "-") take them after an optional ':'.
	Args []Arg
	// Source says where Option Argument suggestions come from besides Args.
	Source ArgSource
	// SubOptions follow the first ,-part of the argument (gorm index:name,sort:desc).
	SubOptions []Option
}

// Arg is a suggested Option Argument.
type Arg struct {
	Value  string
	Detail string
	// For limits the argument to fields of these classes. Fields whose type
	// is not a plain scalar (slices, maps, named types) see every argument.
	For Class
}

// ArgSource says where Option Argument suggestions come from.
type ArgSource int

const (
	FromArgs          ArgSource = iota // the option's Args
	OneField                           // a sibling field: eqfield=Password
	FieldList                          // sibling fields: required_with=Email Phone
	FieldPairs                         // field/value pairs: required_if=Status active Role admin
	RelationFields                     // this struct's and the related struct's fields: foreignKey:OwnerID
	RelatedFields                      // the related struct's fields: polymorphicType:OwnerType
	PolymorphicPrefix                  // X where the related struct has XID and XType: polymorphic:Owner
	Constants                          // typed constants of the field's type: oneof=active inactive
)

// TakesArg reports whether the option is followed by an argument.
func (o Option) TakesArg() bool {
	n := len(o.Name)
	return n > 1 && (o.Name[n-1] == '=' || o.Name[n-1] == ':')
}

var jsonOptions = []Option{
	{Name: "omitempty", Detail: "omit if empty value"},
	{Name: "omitzero", Detail: "omit if zero value (Go 1.24+)"},
	{Name: "string", Detail: "encode number/bool as JSON string", For: String | Number | Bool},
}

var yamlOptions = []Option{
	{Name: "omitempty", Detail: "omit if empty value"},
	{Name: "flow", Detail: "use flow style", For: Slice | Map | Struct},
	{Name: "inline", Detail: "inline struct or map fields", For: Struct | Map},
}

var xmlOptions = []Option{
	{Name: "attr", Detail: "encode as XML attribute", For: String | Number | Bool | Slice | Map | Time},
	{Name: "chardata", Detail: "encode as character data", For: String | Number | Bool | Slice | Map | Time},
	{Name: "cdata", Detail: "encode as CDATA section", For: String | Number | Bool | Slice | Map | Time},
	{Name: "innerxml", Detail: "write raw inner XML", For: String | Number | Bool | Slice | Map | Time},
	{Name: "comment", Detail: "encode as XML comment", For: String | Number | Bool | Slice | Map | Time},
	{Name: "any", Detail: "match any unmatched element"},
	{Name: "omitempty", Detail: "omit if empty value"},
}

var tomlOptions = []Option{
	{Name: "omitempty", Detail: "omit if empty value"},
	{Name: "omitzero", Detail: "omit if zero value"},
	{Name: "multiline", Detail: "encode string as multiline", For: String},
	{Name: "inline", Detail: "encode table inline", For: Struct | Map},
}

var bsonOptions = []Option{
	{Name: "omitempty", Detail: "omit if empty value"},
	{Name: "omitzero", Detail: "omit if zero value"},
	{Name: "minsize", Detail: "store int64 as int32 if it fits", For: Integer},
	{Name: "truncate", Detail: "allow lossy float→int decode", For: Integer},
	{Name: "inline", Detail: "inline struct or map fields", For: Struct | Map},
}

var mapstructureOptions = []Option{
	{Name: "squash", Detail: "flatten embedded struct", For: Struct},
	{Name: "remain", Detail: "collect unmapped keys (map field)", For: Map},
	{Name: "omitempty", Detail: "omit if empty value"},
}

// gorm settings are ';'-separated and case-insensitive.
var gormOptions = []Option{
	{Name: "primaryKey", Detail: "primary key"},
	{Name: "type:", Detail: "column data type", Args: pgTypes},
	{Name: "size:", Detail: "column size/length", For: String | Integer | Slice},
	{Name: "not null", Detail: "NOT NULL constraint"},
	{Name: "unique", Detail: "unique constraint"},
	{Name: "default:", Detail: "column default value", Args: []Arg{
		{Value: "gen_random_uuid()", Detail: "random UUID", For: String},
		{Value: "now()", Detail: "current timestamp", For: Time},
		{Value: "CURRENT_TIMESTAMP", Detail: "current timestamp", For: Time},
		{Value: "true", For: Bool},
		{Value: "false", For: Bool},
		{Value: "0", For: Number},
		{Value: "''", Detail: "empty string", For: String},
	}},
	{Name: "index", Detail: "create index"},
	{Name: "index:", Detail: "create named/composite index", SubOptions: indexOptions},
	{Name: "uniqueIndex", Detail: "create unique index"},
	{Name: "uniqueIndex:", Detail: "create named unique index", SubOptions: indexOptions},
	{Name: "autoIncrement", Detail: "auto increment", For: Integer},
	{Name: "autoIncrementIncrement:", Detail: "auto increment step", For: Integer},
	{Name: "autoCreateTime", Detail: "set to now on create (:milli / :nano for ints)", For: Time | Integer, Args: timeUnits},
	{Name: "autoUpdateTime", Detail: "set to now on create/update (:milli / :nano for ints)", For: Time | Integer, Args: timeUnits},
	{Name: "precision:", Detail: "numeric/time precision", For: Float | Time},
	{Name: "scale:", Detail: "numeric scale", For: Float},
	{Name: "comment:", Detail: "column comment"},
	{Name: "check:", Detail: "check constraint"},
	{Name: "serializer:", Detail: "serializer (json, gob, unixtime)", Args: []Arg{
		{Value: "json", Detail: "store as JSON"},
		{Value: "gob", Detail: "store as gob"},
		{Value: "unixtime", Detail: "store time as unix seconds"},
	}},
	{Name: "embedded", Detail: "embed struct fields", For: Struct},
	{Name: "embeddedPrefix:", Detail: "prefix for embedded columns", For: Struct},
	{Name: "foreignKey:", Detail: "foreign key field", For: relation, Source: RelationFields},
	{Name: "references:", Detail: "referenced field", For: relation, Source: RelationFields},
	{Name: "constraint:", Detail: "relation constraint (OnUpdate/OnDelete)", For: relation, Args: []Arg{
		{Value: "OnUpdate:CASCADE,OnDelete:SET NULL"},
		{Value: "OnUpdate:CASCADE,OnDelete:CASCADE"},
		{Value: "OnDelete:SET NULL"},
		{Value: "OnDelete:CASCADE"},
	}},
	{Name: "many2many:", Detail: "join table name", For: StructSlice},
	{Name: "joinForeignKey:", Detail: "join table foreign key", For: StructSlice, Source: RelationFields},
	{Name: "joinReferences:", Detail: "join table reference", For: StructSlice, Source: RelationFields},
	{Name: "polymorphic:", Detail: "polymorphic relation (prefix of XID/XType fields)", For: relation, Source: PolymorphicPrefix},
	{Name: "polymorphicType:", Detail: "related type column field", For: relation, Source: RelatedFields},
	{Name: "polymorphicId:", Detail: "related id column field", For: relation, Source: RelatedFields},
	{Name: "polymorphicValue:", Detail: "value stored in the type column", For: relation},
	{Name: "<-", Detail: "write permission (<-:create, <-:update, <-:false)", Args: []Arg{
		{Value: "create", Detail: "write on create only"},
		{Value: "update", Detail: "write on update only"},
		{Value: "false", Detail: "no write permission"},
	}},
	{Name: "->", Detail: "read-only (->:false disables read)", Args: []Arg{
		{Value: "false", Detail: "no read permission"},
	}},
	{Name: "-", Detail: "ignore field (-:migration, -:all)", Args: []Arg{
		{Value: "migration", Detail: "ignore in migration only"},
		{Value: "all", Detail: "ignore for read, write and migration"},
	}},
}

// relation fields are structs or slices of structs (belongs-to, has-one, has-many, many2many).
const relation = Struct | StructSlice

var timeUnits = []Arg{
	{Value: "milli", Detail: "unix milliseconds (int field)", For: Integer},
	{Value: "nano", Detail: "unix nanoseconds (int field)", For: Integer},
}

// indexOptions follow the index name: index:idx_email,sort:desc,type:btree.
var indexOptions = []Option{
	{Name: "unique", Detail: "unique index"},
	{Name: "sort:", Detail: "column sort order", Args: []Arg{{Value: "desc"}, {Value: "asc"}}},
	{Name: "type:", Detail: "index method", Args: []Arg{
		{Value: "btree"}, {Value: "hash"}, {Value: "gin", Detail: "jsonb, arrays, full text"},
		{Value: "gist"}, {Value: "brin"},
	}},
	{Name: "where:", Detail: "partial index condition"},
	{Name: "composite:", Detail: "composite index id shared across fields"},
	{Name: "priority:", Detail: "column order in composite index"},
	{Name: "expression:", Detail: "index on an expression"},
	{Name: "class:", Detail: "index class"},
	{Name: "length:", Detail: "prefix length"},
	{Name: "option:", Detail: "extra index option"},
	{Name: "collate:", Detail: "collation"},
}

// pgTypes are Postgres column types for gorm's type:, grouped by Go type.
var pgTypes = []Arg{
	{Value: "uuid", For: String},
	{Value: "text", For: String},
	{Value: "varchar(255)", For: String},
	{Value: "char(36)", For: String},
	{Value: "citext", Detail: "case-insensitive text", For: String},
	{Value: "inet", Detail: "IP address", For: String},
	{Value: "bigint", For: Integer},
	{Value: "integer", For: Integer},
	{Value: "smallint", For: Integer},
	{Value: "bigserial", Detail: "auto-increment bigint", For: Integer},
	{Value: "serial", Detail: "auto-increment integer", For: Integer},
	{Value: "numeric(10,2)", For: Float},
	{Value: "double precision", For: Float},
	{Value: "real", For: Float},
	{Value: "timestamptz", Detail: "timestamp with time zone", For: Time},
	{Value: "timestamp", Detail: "timestamp without time zone", For: Time},
	{Value: "date", For: Time},
	{Value: "time", For: Time},
	{Value: "boolean", For: Bool},
	{Value: "jsonb", For: Slice | Map},
	{Value: "json", For: Slice | Map},
	{Value: "bytea", Detail: "binary data", For: Slice | Map},
	{Value: "text[]", Detail: "text array", For: Slice | Map},
}

// env options for sethvargo/go-envconfig, written after ", ".
var envOptions = []Option{
	{Name: "default=", Detail: "value when unset"},
	{Name: "required", Detail: "error if unset"},
	{Name: "prefix=", Detail: "prefix for nested struct keys"},
	{Name: "overwrite", Detail: "overwrite non-zero field"},
	{Name: "delimiter=", Detail: "slice/map entry separator (default ,)"},
	{Name: "separator=", Detail: "map key/value separator (default :)"},
	{Name: "noinit", Detail: "don't init nil fields when unset"},
	{Name: "decodeunset", Detail: "run decoder even when unset"},
}

const (
	sized   = String | Number | Slice | Map // len/min/max semantics
	ordered = sized | Time                  // gt/lt semantics
)

// validate rules for go-playground/validator.
var validateOptions = []Option{
	// any type
	{Name: "required", Detail: "must not be zero value"},
	{Name: "omitempty", Detail: "skip rules if empty"},
	{Name: "omitnil", Detail: "skip rules if nil"},
	{Name: "isdefault", Detail: "must be zero value"},
	{Name: "required_if=", Detail: "required if Field value …", Source: FieldPairs},
	{Name: "required_unless=", Detail: "required unless Field value …", Source: FieldPairs},
	{Name: "required_with=", Detail: "required if any field present", Source: FieldList},
	{Name: "required_with_all=", Detail: "required if all fields present", Source: FieldList},
	{Name: "required_without=", Detail: "required if any field absent", Source: FieldList},
	{Name: "required_without_all=", Detail: "required if all fields absent", Source: FieldList},
	{Name: "excluded_if=", Detail: "must be empty if Field value …", Source: FieldPairs},
	{Name: "excluded_unless=", Detail: "must be empty unless Field value …", Source: FieldPairs},
	{Name: "excluded_with=", Detail: "must be empty if field present", Source: FieldList},
	{Name: "excluded_without=", Detail: "must be empty if field absent", Source: FieldList},
	{Name: "eqfield=", Detail: "equal to another field", Source: OneField},
	{Name: "nefield=", Detail: "not equal to another field", Source: OneField},
	{Name: "gtfield=", Detail: "greater than another field", For: ordered, Source: OneField},
	{Name: "gtefield=", Detail: "≥ another field", For: ordered, Source: OneField},
	{Name: "ltfield=", Detail: "less than another field", For: ordered, Source: OneField},
	{Name: "ltefield=", Detail: "≤ another field", For: ordered, Source: OneField},

	// size / comparison
	{Name: "len=", Detail: "exact length / value", For: sized},
	{Name: "min=", Detail: "minimum length / value", For: sized},
	{Name: "max=", Detail: "maximum length / value", For: sized},
	{Name: "eq=", Detail: "equal to", For: sized | Bool},
	{Name: "ne=", Detail: "not equal to", For: sized | Bool},
	{Name: "gt", Detail: "after now", For: Time},
	{Name: "gte", Detail: "now or after", For: Time},
	{Name: "lt", Detail: "before now", For: Time},
	{Name: "lte", Detail: "now or before", For: Time},
	{Name: "gt=", Detail: "greater than", For: sized},
	{Name: "gte=", Detail: "greater than or equal", For: sized},
	{Name: "lt=", Detail: "less than", For: sized},
	{Name: "lte=", Detail: "less than or equal", For: sized},
	{Name: "oneof=", Detail: "one of space-separated values", For: String | Number, Source: Constants},

	// collections
	{Name: "dive", Detail: "apply following rules to elements", For: Slice | Map},
	{Name: "unique", Detail: "no duplicate elements", For: Slice | Map},
	{Name: "keys", Detail: "start rules for map keys (end with endkeys)", For: Map},
	{Name: "endkeys", Detail: "end of map key rules", For: Map},

	// strings
	{Name: "email", Detail: "email address", For: String},
	{Name: "url", Detail: "URL", For: String},
	{Name: "http_url", Detail: "HTTP(S) URL", For: String},
	{Name: "uri", Detail: "URI", For: String},
	{Name: "uuid", Detail: "UUID", For: String},
	{Name: "uuid4", Detail: "UUID v4", For: String},
	{Name: "ulid", Detail: "ULID", For: String},
	{Name: "alpha", Detail: "ASCII letters only", For: String},
	{Name: "alphanum", Detail: "ASCII letters and digits", For: String},
	{Name: "alphaunicode", Detail: "unicode letters only", For: String},
	{Name: "numeric", Detail: "numeric string", For: String},
	{Name: "number", Detail: "digits only", For: String},
	{Name: "boolean", Detail: "parsable as bool", For: String},
	{Name: "hexadecimal", Detail: "hexadecimal string", For: String},
	{Name: "hexcolor", Detail: "hex color (#fff)", For: String},
	{Name: "rgb", Detail: "rgb() color", For: String},
	{Name: "lowercase", Detail: "all lowercase", For: String},
	{Name: "uppercase", Detail: "all uppercase", For: String},
	{Name: "ascii", Detail: "ASCII only", For: String},
	{Name: "printascii", Detail: "printable ASCII only", For: String},
	{Name: "contains=", Detail: "contains substring", For: String},
	{Name: "containsany=", Detail: "contains any of chars", For: String},
	{Name: "excludes=", Detail: "does not contain substring", For: String},
	{Name: "startswith=", Detail: "starts with", For: String},
	{Name: "endswith=", Detail: "ends with", For: String},
	{Name: "json", Detail: "valid JSON", For: String},
	{Name: "jwt", Detail: "JSON Web Token", For: String},
	{Name: "base64", Detail: "base64 string", For: String},
	{Name: "base64url", Detail: "base64url string", For: String},
	{Name: "ip", Detail: "IP address", For: String},
	{Name: "ipv4", Detail: "IPv4 address", For: String},
	{Name: "ipv6", Detail: "IPv6 address", For: String},
	{Name: "cidr", Detail: "CIDR notation", For: String},
	{Name: "mac", Detail: "MAC address", For: String},
	{Name: "hostname", Detail: "hostname (RFC 952)", For: String},
	{Name: "fqdn", Detail: "fully qualified domain name", For: String},
	{Name: "e164", Detail: "E.164 phone number", For: String},
	{Name: "datetime=", Detail: "matches Go time layout", For: String, Args: []Arg{
		{Value: "2006-01-02", Detail: "date"},
		{Value: "2006-01-02T15:04:05Z07:00", Detail: "RFC 3339"},
		{Value: "15:04", Detail: "time of day"},
	}},
	{Name: "timezone", Detail: "IANA time zone", For: String},
	{Name: "iso3166_1_alpha2", Detail: "ISO 3166 country code", For: String},
	{Name: "semver", Detail: "semantic version", For: String},
	{Name: "latitude", Detail: "latitude", For: String | Number},
	{Name: "longitude", Detail: "longitude", For: String | Number},
	{Name: "file", Detail: "existing file path", For: String},
	{Name: "dir", Detail: "existing directory path", For: String},
	{Name: "cron", Detail: "cron expression", For: String},
	{Name: "credit_card", Detail: "credit card number", For: String},
}
