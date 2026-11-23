package main

import (
	"fmt"
	"slices"
	"strings"
	"unicode/utf16"

	"go-tags-ls/catalog"
	"go-tags-ls/naming"
	"go-tags-ls/pkgindex"
	"go-tags-ls/tagctx"
)

type lspRange struct {
	Start position `json:"start"`
	End   position `json:"end"`
}

type textEdit struct {
	Range   lspRange `json:"range"`
	NewText string   `json:"newText"`
}

type completionItem struct {
	Label            string    `json:"label"`
	Kind             int       `json:"kind,omitempty"`
	Detail           string    `json:"detail,omitempty"`
	SortText         string    `json:"sortText,omitempty"`
	FilterText       string    `json:"filterText,omitempty"`
	InsertText       string    `json:"insertText,omitempty"`
	InsertTextFormat int       `json:"insertTextFormat,omitempty"`
	TextEdit         *textEdit `json:"textEdit,omitempty"`
}

const (
	completionKindProperty  = 10
	completionKindValue     = 12
	insertTextFormatPlain   = 1
	insertTextFormatSnippet = 2
)

// request is one completion: the tag context plus what we know about the
// field's type and the surrounding package.
type request struct {
	keys   []catalog.Key
	idx    *pkgindex.Index
	ctx    tagctx.Context
	cursor position
	class  catalog.Class // the field type's class, resolved through idx
	items  []completionItem
}

// complete returns suggestions for the Struct Tag under the cursor.
func (s *server) complete(p completionParams) []completionItem {
	text, ok := s.docs[p.TextDocument.URI]
	if !ok {
		return []completionItem{}
	}
	col, ok := byteColumn(text, p.Position)
	if !ok {
		return []completionItem{}
	}
	ctx, ok := tagctx.At(text, p.Position.Line, col)
	if !ok || ctx.Position == tagctx.None {
		return []completionItem{}
	}
	idx := s.packageIndex(p.TextDocument.URI, text)
	rq := &request{
		keys:   s.keys,
		idx:    idx,
		ctx:    ctx,
		cursor: p.Position,
		class:  catalog.Classify(ctx.FieldType, idx.Resolve),
		items:  []completionItem{},
	}
	if ctx.Position == tagctx.Key {
		rq.keyItems()
	} else {
		rq.valueItems()
	}
	return rq.items
}

func (rq *request) field() string { return rq.ctx.FieldNames[0] } // grouped fields: first name

// isStruct is true only for fields known to be structs (not unknown types).
func (rq *request) isStruct() bool { return rq.class == catalog.Struct }

func (rq *request) add(kind int, label, detail string, r lspRange, newText string, format int) {
	rq.items = append(rq.items, completionItem{
		Label:            label,
		Kind:             kind,
		Detail:           detail,
		SortText:         fmt.Sprintf("%03d", len(rq.items)),
		FilterText:       label,
		InsertTextFormat: format,
		TextEdit:         &textEdit{Range: r, NewText: newText},
	})
}

// envPrefix is go-envconfig's nested-struct form: ", prefix=DATABASE_".
func (rq *request) envPrefix() string {
	return ", prefix=" + naming.Apply(naming.ScreamingSnake, rq.field()) + "_"
}

// keyItems suggests Tag Keys not yet in the tag. Picking one inserts the
// whole pair, e.g. json:"user_id", with the value selected.
func (rq *request) keyItems() {
	r := rangeBack(rq.cursor, rq.ctx.KeyPartial)
	for _, k := range rq.keys {
		if slices.Contains(rq.ctx.ExistingKeys, k.Name) {
			continue
		}
		var snippet, detail string
		switch {
		case k.Kind == catalog.Rules:
			snippet = k.Name + `:"$1"`
			detail = k.Library
		case k.Name == "env" && rq.isStruct():
			prefix := strings.TrimPrefix(rq.envPrefix(), ", prefix=")
			snippet = fmt.Sprintf(`env:", prefix=${1:%s}"`, escapeSnippet(prefix))
			detail = fmt.Sprintf(`%s · env:", prefix=%s"`, k.Library, prefix)
		default:
			v := k.Value(rq.field())
			snippet = fmt.Sprintf(`%s:"${1:%s}"`, k.Name, escapeSnippet(v))
			detail = fmt.Sprintf(`%s · %s:"%s"`, k.Library, k.Name, v)
		}
		rq.items = append(rq.items, completionItem{
			Label:            k.Name,
			Kind:             completionKindProperty,
			Detail:           detail,
			SortText:         fmt.Sprintf("%02d", len(rq.items)),
			FilterText:       k.Name,
			InsertTextFormat: insertTextFormatSnippet,
			TextEdit:         &textEdit{Range: r, NewText: snippet},
		})
	}
}

// valueItems suggests inside a Tag Key's quotes. The value is split into
// parts by the key's separators. Named keys take the derived name in their
// first part (gorm: in any part, as "column:..."); every other part takes
// Tag Options, or Option Arguments once an option's ':' / '=' is typed.
func (rq *request) valueItems() {
	k, ok := catalog.Find(rq.keys, rq.ctx.Key)
	if !ok {
		return
	}
	parts := k.Split(rq.ctx.ValuePrefix)
	current, done := parts[len(parts)-1], parts[:len(parts)-1]
	typed := strings.TrimLeft(current, " ")
	r := rangeBack(rq.cursor, typed)

	if o, arg, ok := k.ArgOption(typed); ok && (k.Kind == catalog.Rules || len(done) > 0 || k.Prefix != "") {
		rq.argItems(o, arg)
		return
	}

	nameItems := func() {
		if k.Name == "env" && rq.isStruct() {
			p := rq.envPrefix()
			rq.add(completionKindValue, p, "nested struct prefix (go-envconfig)", r, p, insertTextFormatPlain)
			return
		}
		v := k.Value(rq.field())
		rq.add(completionKindValue, v, fmt.Sprintf("%s from %s (%s)", k.Name, rq.field(), k.Style), r, v, insertTextFormatPlain)
		if k.Skip {
			rq.add(completionKindValue, "-", "ignore this field", r, "-", insertTextFormatPlain)
		}
	}
	if k.Kind == catalog.Named {
		if k.Prefix == "" && len(done) == 0 {
			nameItems() // first part is the name; options come after a separator
			return
		}
		if k.Prefix != "" && !hasPrefixPart(done, k.Prefix) {
			nameItems()
		}
	}

	// After "dive", rules apply to elements, so earlier rules may repeat.
	if i := slices.Index(done, "dive"); i >= 0 && k.Name == "validate" {
		done = done[i+1:]
	}
	lead := ""
	if k.SpaceAfterSep && len(parts) > 1 && current == "" {
		lead = " " // env:"PORT,| -> env:"PORT, required
	}
	for _, o := range k.Options {
		if k.Used(o, done) || (o.For != 0 && o.For&rq.class == 0) {
			continue
		}
		rq.addOption(o, lead, r)
	}
}

func (rq *request) addOption(o catalog.Option, lead string, r lspRange) {
	if o.TakesArg() {
		rq.add(completionKindValue, o.Name, o.Detail, r, lead+escapeSnippet(o.Name)+"$1", insertTextFormatSnippet)
	} else {
		rq.add(completionKindValue, o.Name, o.Detail, r, lead+o.Name, insertTextFormatPlain)
	}
}

// argItems suggests Option Arguments for option o, whose argument text so far is arg.
func (rq *request) argItems(o catalog.Option, arg string) {
	if len(o.SubOptions) > 0 {
		rq.subOptionItems(o, arg)
		return
	}
	// Field and constant lists are space-separated tokens; replace only the last one.
	tokens := strings.Split(arg, " ")
	token := tokens[len(tokens)-1]
	r := rangeBack(rq.cursor, token)
	value := func(label, detail string) {
		rq.add(completionKindValue, label, detail, r, label, insertTextFormatPlain)
	}

	switch o.Source {
	case catalog.OneField:
		if len(tokens) == 1 {
			for _, f := range rq.ctx.SiblingFields {
				value(f, "field")
			}
		}
		return
	case catalog.FieldList:
		for _, f := range rq.ctx.SiblingFields {
			if !slices.Contains(tokens[:len(tokens)-1], f) {
				value(f, "field")
			}
		}
		return
	case catalog.FieldPairs: // Field value Field value ...
		if len(tokens)%2 == 1 {
			for _, f := range rq.ctx.SiblingFields {
				value(f, "field")
			}
		}
		return
	case catalog.Constants:
		rq.constantItems(tokens, r)
		return
	}

	r = rangeBack(rq.cursor, arg) // other arguments may contain spaces ("double precision")
	value = func(label, detail string) {
		rq.add(completionKindValue, label, detail, r, label, insertTextFormatPlain)
	}
	switch o.Source {
	case catalog.RelationFields:
		rq.relationItems(value, true)
		return
	case catalog.RelatedFields:
		rq.relationItems(value, false)
		return
	case catalog.PolymorphicPrefix:
		name := pkgindex.ElemTypeName(rq.ctx.FieldType)
		fields, _ := rq.idx.Struct(name)
		for _, f := range fields {
			if p, ok := strings.CutSuffix(f, "ID"); ok && p != "" && slices.Contains(fields, p+"Type") {
				value(p, fmt.Sprintf("%s has %sID and %sType", name, p, p))
			}
		}
		return
	}
	for _, a := range o.Args {
		if a.Allows(rq.class) {
			value(a.Value, a.Detail)
		}
	}
}

// relationItems suggests gorm relation fields: this struct's fields (when own
// is set) and the related struct's, each labelled with its struct. A field in
// both is listed once.
func (rq *request) relationItems(value func(label, detail string), own bool) {
	type entry struct {
		name    string
		structs []string
	}
	var entries []*entry
	seen := map[string]*entry{}
	addField := func(name, structName string) {
		if e, ok := seen[name]; ok {
			e.structs = append(e.structs, structName)
			return
		}
		e := &entry{name: name, structs: []string{structName}}
		seen[name] = e
		entries = append(entries, e)
	}
	if own {
		this := rq.ctx.StructName
		if this == "" {
			this = "this struct"
		}
		for _, f := range rq.ctx.SiblingFields {
			addField(f, this)
		}
	}
	related := pkgindex.ElemTypeName(rq.ctx.FieldType)
	if fields, ok := rq.idx.Struct(related); ok && related != rq.ctx.StructName {
		for _, f := range fields {
			addField(f, related)
		}
	}
	for _, e := range entries {
		value(e.name, strings.Join(e.structs, " / ")+" field")
	}
}

// constantItems suggests the typed constants of the field's type for oneof=:
// all of them at once, then each one not yet listed.
func (rq *request) constantItems(tokens []string, r lspRange) {
	typeName := pkgindex.ElemTypeName(rq.ctx.FieldType)
	if rq.idx == nil || typeName == "" {
		return
	}
	var values, names []string
	for _, c := range rq.idx.Consts[typeName] {
		if c.Value == "" || strings.ContainsAny(c.Value, " \t") || slices.Contains(values, c.Value) {
			continue // oneof= can't express empty or spaced values
		}
		values = append(values, c.Value)
		names = append(names, c.Name)
	}
	listed := tokens[:len(tokens)-1]
	if len(listed) == 0 && len(values) > 1 {
		all := strings.Join(values, " ")
		rq.add(completionKindValue, all, fmt.Sprintf("all %d %s values", len(values), typeName), r, all, insertTextFormatPlain)
	}
	for i, v := range values {
		if !slices.Contains(listed, v) {
			rq.add(completionKindValue, v, names[i], r, v, insertTextFormatPlain)
		}
	}
}

// subOptionItems handles options whose argument has its own ,-separated
// options after a name: gorm index:idx_email,sort:desc,type:btree.
func (rq *request) subOptionItems(o catalog.Option, arg string) {
	parts := strings.Split(arg, ",")
	if len(parts) == 1 {
		return // still typing the index name
	}
	cur, done := parts[len(parts)-1], parts[1:len(parts)-1]
	if i := strings.IndexByte(cur, ':'); i >= 0 {
		r := rangeBack(rq.cursor, cur[i+1:])
		for _, sub := range o.SubOptions {
			if sub.Name == cur[:i+1] {
				for _, a := range sub.Args {
					if a.Allows(rq.class) {
						rq.add(completionKindValue, a.Value, a.Detail, r, a.Value, insertTextFormatPlain)
					}
				}
			}
		}
		return
	}
	r := rangeBack(rq.cursor, cur)
	for _, sub := range o.SubOptions {
		if !sub.UsedIn(done, true) {
			rq.addOption(sub, "", r)
		}
	}
}

func hasPrefixPart(parts []string, prefix string) bool {
	for _, p := range parts {
		if strings.HasPrefix(strings.TrimSpace(p), prefix) {
			return true
		}
	}
	return false
}

// rangeBack is the range covering typed (just before the cursor) on the cursor's line.
func rangeBack(cursor position, typed string) lspRange {
	start := cursor
	start.Character -= len(utf16.Encode([]rune(typed)))
	return lspRange{Start: start, End: cursor}
}

// escapeSnippet escapes snippet metacharacters in placeholder text.
func escapeSnippet(s string) string {
	return strings.NewReplacer(`\`, `\\`, `$`, `\$`, `}`, `\}`).Replace(s)
}
