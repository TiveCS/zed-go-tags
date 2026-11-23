package main

import (
	"fmt"
	"slices"
	"strings"
	"unicode/utf16"

	"go-tags-ls/catalog"
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

// complete returns suggestions for the Struct Tag under the cursor.
func (s *server) complete(p completionParams) []completionItem {
	items := []completionItem{}
	text, ok := s.docs[p.TextDocument.URI]
	if !ok {
		return items
	}
	col, ok := byteColumn(text, p.Position)
	if !ok {
		return items
	}
	ctx, ok := tagctx.At(text, p.Position.Line, col)
	if !ok {
		return items
	}
	switch ctx.Position {
	case tagctx.Key:
		return keyItems(s.keys, ctx, p.Position)
	case tagctx.Value:
		return valueItems(s.keys, ctx, p.Position)
	}
	return items
}

// keyItems suggests Tag Keys not yet in the tag. Picking one inserts the
// whole pair, e.g. json:"user_id", with the value selected.
func keyItems(keys []catalog.Key, ctx tagctx.Context, cursor position) []completionItem {
	items := []completionItem{}
	field := ctx.FieldNames[0] // grouped fields: use the first name
	r := rangeBack(cursor, ctx.KeyPartial)
	for i, k := range keys {
		if slices.Contains(ctx.ExistingKeys, k.Name) {
			continue
		}
		var snippet, detail string
		if k.Kind == catalog.Rules {
			snippet = k.Name + `:"$1"`
			detail = k.Library
		} else {
			v := k.Value(field)
			snippet = fmt.Sprintf(`%s:"${1:%s}"`, k.Name, escapeSnippet(v))
			detail = fmt.Sprintf(`%s · %s:"%s"`, k.Library, k.Name, v)
		}
		items = append(items, completionItem{
			Label:            k.Name,
			Kind:             completionKindProperty,
			Detail:           detail,
			SortText:         fmt.Sprintf("%02d", i),
			FilterText:       k.Name,
			InsertTextFormat: insertTextFormatSnippet,
			TextEdit:         &textEdit{Range: r, NewText: snippet},
		})
	}
	return items
}

// valueItems suggests the Tag Value derived from the field name.
//
// Inside a Tag Key's quotes the value is split into parts by the key's
// separators. Named keys take the derived name in their first part (gorm:
// in any part, as "column:..."); every other part takes Tag Options.
func valueItems(keys []catalog.Key, ctx tagctx.Context, cursor position) []completionItem {
	items := []completionItem{}
	k, ok := catalog.Find(keys, ctx.Key)
	if !ok {
		return items
	}
	parts := k.Split(ctx.ValuePrefix)
	current, done := parts[len(parts)-1], parts[:len(parts)-1]
	typed := strings.TrimLeft(current, " ")
	r := rangeBack(cursor, typed)
	field := ctx.FieldNames[0]

	add := func(label, detail, newText string, format int) {
		items = append(items, completionItem{
			Label:            label,
			Kind:             completionKindValue,
			Detail:           detail,
			SortText:         fmt.Sprintf("%03d", len(items)),
			FilterText:       label,
			InsertTextFormat: format,
			TextEdit:         &textEdit{Range: r, NewText: newText},
		})
	}
	nameItems := func() {
		v := k.Value(field)
		add(v, fmt.Sprintf("%s from %s (%s)", k.Name, field, k.Style), v, insertTextFormatPlain)
		if k.Skip {
			add("-", "ignore this field", "-", insertTextFormatPlain)
		}
	}

	if o, arg, ok := k.ArgOption(typed); ok && (k.Kind == catalog.Rules || len(done) > 0 || k.Prefix != "") {
		return argItems(o, arg, ctx, cursor)
	}

	if k.Kind == catalog.Named {
		if k.Prefix == "" && len(done) == 0 {
			nameItems() // first part is the name; options come after a separator
			return items
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
	class := catalog.Classify(ctx.FieldType)
	for _, o := range k.Options {
		if k.Used(o, done) || (o.For != 0 && o.For&class == 0) {
			continue
		}
		if o.TakesArg() {
			add(o.Name, o.Detail, lead+escapeSnippet(o.Name)+"$1", insertTextFormatSnippet)
		} else {
			add(o.Name, o.Detail, lead+o.Name, insertTextFormatPlain)
		}
	}
	return items
}

// argItems suggests Option Arguments for option o, whose argument text so far is arg.
func argItems(o catalog.Option, arg string, ctx tagctx.Context, cursor position) []completionItem {
	items := []completionItem{}
	// Field-reference arguments are space-separated tokens; replace only the last one.
	tokens := strings.Split(arg, " ")
	token := tokens[len(tokens)-1]
	r := rangeBack(cursor, token)
	add := func(label, detail string) {
		items = append(items, completionItem{
			Label:            label,
			Kind:             completionKindValue,
			Detail:           detail,
			SortText:         fmt.Sprintf("%03d", len(items)),
			FilterText:       label,
			InsertTextFormat: insertTextFormatPlain,
			TextEdit:         &textEdit{Range: r, NewText: label},
		})
	}

	switch o.FieldRef {
	case catalog.OneField:
		if len(tokens) == 1 {
			for _, f := range ctx.SiblingFields {
				add(f, "field")
			}
		}
		return items
	case catalog.FieldList:
		for _, f := range ctx.SiblingFields {
			if !slices.Contains(tokens[:len(tokens)-1], f) {
				add(f, "field")
			}
		}
		return items
	case catalog.FieldPairs: // Field value Field value ...
		if len(tokens)%2 == 1 {
			for _, f := range ctx.SiblingFields {
				add(f, "field")
			}
		}
		return items
	}

	class := catalog.Classify(ctx.FieldType)
	token = arg // plain arguments may contain spaces ("double precision")
	r = rangeBack(cursor, token)
	for _, a := range o.Args {
		if a.Allows(class) {
			add(a.Value, a.Detail)
		}
	}
	return items
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
