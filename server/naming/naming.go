// Package naming turns Go field names into Tag Values in a Naming Style.
package naming

import (
	"strings"
	"unicode"
)

// Style is a Naming Style.
type Style string

const (
	Snake          Style = "snake_case"      // user_id
	Camel          Style = "camelCase"       // userId
	Pascal         Style = "PascalCase"      // UserId
	ScreamingSnake Style = "SCREAMING_SNAKE" // USER_ID
	Kebab          Style = "kebab-case"      // user-id
	Header         Style = "Header-Case"     // User-Id
)

// Styles lists every supported Naming Style.
var Styles = []Style{Snake, Camel, Pascal, ScreamingSnake, Kebab, Header}

// Valid reports whether s is a supported Naming Style.
func Valid(s Style) bool {
	for _, v := range Styles {
		if v == s {
			return true
		}
	}
	return false
}

// Apply converts a Go identifier to the given Naming Style.
// Acronyms are treated as one word: UserID -> user_id / userId, HTTPServer -> http_server / httpServer.
func Apply(style Style, name string) string {
	words := Words(name)
	for i, w := range words {
		words[i] = strings.ToLower(w)
	}
	switch style {
	case Camel:
		for i := 1; i < len(words); i++ {
			words[i] = title(words[i])
		}
		return strings.Join(words, "")
	case Pascal:
		for i := range words {
			words[i] = title(words[i])
		}
		return strings.Join(words, "")
	case ScreamingSnake:
		return strings.ToUpper(strings.Join(words, "_"))
	case Kebab:
		return strings.Join(words, "-")
	case Header:
		for i := range words {
			words[i] = title(words[i])
		}
		return strings.Join(words, "-")
	default:
		return strings.Join(words, "_")
	}
}

// Words splits a Go identifier into words.
//
//	UserID      -> User ID
//	HTTPServer  -> HTTP Server
//	userIDs     -> user IDs
//	Address2    -> Address2
//	V2API       -> V2 API
//	created_at  -> created at
func Words(name string) []string {
	var words []string
	rs := []rune(name)
	start := 0
	flush := func(end int) {
		if end > start {
			words = append(words, string(rs[start:end]))
		}
		start = end
	}
	for i := 0; i < len(rs); i++ {
		r := rs[i]
		if r == '_' || r == '-' {
			flush(i)
			start = i + 1
			continue
		}
		if i == start || !unicode.IsUpper(r) {
			continue
		}
		prev := rs[i-1]
		switch {
		case unicode.IsLower(prev) || unicode.IsDigit(prev):
			// userId: boundary before I
			flush(i)
		case unicode.IsUpper(prev) && i+1 < len(rs) && unicode.IsLower(rs[i+1]) && !pluralS(rs, i):
			// HTTPServer: boundary before S (last upper of an acronym run)
			flush(i)
		}
	}
	flush(len(rs))
	return words
}

// pluralS reports whether rs[i+1] is the plural "s" of an acronym ending at rs[i]
// (IDs, URLs), so the acronym stays one word.
func pluralS(rs []rune, i int) bool {
	return rs[i+1] == 's' && (i+2 == len(rs) || !unicode.IsLower(rs[i+2]))
}

func title(w string) string {
	if w == "" {
		return w
	}
	rs := []rune(w)
	rs[0] = unicode.ToUpper(rs[0])
	return string(rs)
}
