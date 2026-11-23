package tagctx

// Struct Tag scanner following the reflect.StructTag convention:
// space-separated key:"value" pairs, values may contain backslash escapes.

type scanState int

const (
	stBetween scanState = iota // before a key (start or after whitespace)
	stKey                      // reading a Tag Key
	stColon                    // read `key:`, expecting an opening quote
	stValue                    // inside the quotes of a value
	stBad                      // malformed segment; skip to next whitespace
)

type pair struct {
	key      string
	keyStart int // byte offset of the key in the tag content
}

type scanResult struct {
	pairs    []pair    // complete (and, at end, unterminated) key:"value" pairs
	state    scanState // state at end of input
	key      string    // current key while in stColon / stValue
	keyStart int       // start of the key being read / owning the value
	valStart int       // start of value content while in stValue
}

func scan(s string) scanResult {
	var r scanResult
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch r.state {
		case stBetween:
			if !isSpace(c) {
				r.keyStart = i
				r.state = stKey
				if c == ':' {
					r.state = stBad
				}
			}
		case stKey:
			switch {
			case c == ':':
				r.key = s[r.keyStart:i]
				r.state = stColon
			case isSpace(c):
				r.state = stBetween // bare word without value: drop it
			}
		case stColon:
			switch {
			case c == '"':
				r.valStart = i + 1
				r.state = stValue
			case isSpace(c):
				r.state = stBetween
			default:
				r.state = stBad
			}
		case stValue:
			switch c {
			case '\\':
				i++ // skip escaped char
			case '"':
				r.pairs = append(r.pairs, pair{key: r.key, keyStart: r.keyStart})
				r.state = stBetween
			}
		case stBad:
			if isSpace(c) {
				r.state = stBetween
			}
		}
	}
	if r.state == stValue {
		r.pairs = append(r.pairs, pair{key: r.key, keyStart: r.keyStart})
	}
	return r
}

func isSpace(c byte) bool { return c == ' ' || c == '\t' }
