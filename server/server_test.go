package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// session feeds LSP frames to a fresh server and returns its replies by id.
func session(t *testing.T, reqs ...map[string]any) map[string]*message {
	t.Helper()
	var in bytes.Buffer
	for _, r := range reqs {
		r["jsonrpc"] = "2.0"
		body, _ := json.Marshal(r)
		in.WriteString("Content-Length: ")
		in.WriteString(itoa(len(body)))
		in.WriteString("\r\n\r\n")
		in.Write(body)
	}
	var out bytes.Buffer
	if err := newServer().serve(bufio.NewReader(&in), &out); err != nil {
		t.Fatalf("serve: %v", err)
	}
	replies := map[string]*message{}
	r := bufio.NewReader(&out)
	for {
		msg, err := readMessage(r)
		if err != nil {
			break
		}
		replies[string(msg.ID)] = msg
	}
	return replies
}

func itoa(n int) string { b, _ := json.Marshal(n); return string(b) }

func completionLabels(t *testing.T, msg *message) []string {
	t.Helper()
	raw, _ := json.Marshal(msg.Result)
	var items []completionItem
	if err := json.Unmarshal(raw, &items); err != nil {
		t.Fatalf("decode items %s: %v", raw, err)
	}
	var labels []string
	for _, it := range items {
		labels = append(labels, it.Label)
	}
	return labels
}

const src = "package x\n\ntype User struct {\n\tID string `\n\tName string\n}\n"

func open() map[string]any {
	return map[string]any{"method": "textDocument/didOpen", "params": map[string]any{
		"textDocument": map[string]any{"uri": "file:///x.go", "languageId": "go", "version": 1, "text": src},
	}}
}

func complete(id, line, char int) map[string]any {
	return map[string]any{"id": id, "method": "textDocument/completion", "params": map[string]any{
		"textDocument": map[string]any{"uri": "file:///x.go"},
		"position":     map[string]any{"line": line, "character": char},
	}}
}

func TestInitializeAdvertisesTriggers(t *testing.T) {
	replies := session(t, map[string]any{"id": 1, "method": "initialize", "params": map[string]any{}})
	raw, _ := json.Marshal(replies["1"].Result)
	for _, want := range []string{`"completionProvider"`, "\"`\""} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("initialize result %s missing %s", raw, want)
		}
	}
}

func TestCompletionInsideBacktick(t *testing.T) {
	// line 3 is "\tID string `" - cursor after the backtick (char 12).
	replies := session(t, open(), complete(1, 3, 12))
	got := completionLabels(t, replies["1"])
	if len(got) != 14 || got[0] != "json" {
		t.Fatalf("labels = %v, want 14 keys starting with json", got)
	}
}

func TestNoCompletionOutsideTag(t *testing.T) {
	// cursor before the backtick (char 11) and on a line without one.
	replies := session(t, open(), complete(1, 3, 11), complete(2, 4, 5))
	for _, id := range []string{"1", "2"} {
		if got := completionLabels(t, replies[id]); len(got) != 0 {
			t.Errorf("id %s labels = %v, want none", id, got)
		}
	}
}

func TestByteColumnUTF16(t *testing.T) {
	// "😀" is 2 UTF-16 units / 4 bytes; character 3 lands after "😀a" (byte 5).
	got, ok := byteColumn("😀ab", position{Line: 0, Character: 3})
	if !ok || got != 5 {
		t.Fatalf("byteColumn = %d, %v", got, ok)
	}
}

func TestShutdownExit(t *testing.T) {
	replies := session(t,
		map[string]any{"id": 1, "method": "shutdown"},
		map[string]any{"method": "exit"},
		complete(2, 0, 0), // after exit: must not be answered
	)
	if _, ok := replies["1"]; !ok {
		t.Fatal("no shutdown reply")
	}
	if _, ok := replies["2"]; ok {
		t.Fatal("server answered after exit")
	}
}

func TestSettingsApplyToCompletion(t *testing.T) {
	src := "package x\n\ntype U struct {\n\tUserID string `json:\"\"`\n\tName string `\n}\n"
	replies, notes := sessionWithNotes(t,
		map[string]any{"id": 1, "method": "initialize", "params": map[string]any{
			"initializationOptions": map[string]any{
				"naming":      map[string]any{"json": "camelCase"},
				"custom_keys": map[string]any{"mytag": "kebab-case", "bad key": "snake_case"},
			},
		}},
		map[string]any{"method": "initialized", "params": map[string]any{}},
		map[string]any{"method": "textDocument/didOpen", "params": map[string]any{
			"textDocument": map[string]any{"uri": "file:///x.go", "languageId": "go", "version": 1, "text": src},
		}},
		complete(2, 3, 22), // inside json:"|"
		complete(3, 4, 14), // key slot on Name
	)
	if got := completionLabels(t, replies["2"]); len(got) == 0 || got[0] != "userId" {
		t.Errorf("json value = %v, want userId (camelCase setting)", got)
	}
	keys := completionLabels(t, replies["3"])
	if keys[len(keys)-1] != "mytag" {
		t.Errorf("custom key missing at end: %v", keys)
	}
	if len(notes) != 1 || notes[0].Method != "window/showMessage" || !strings.Contains(string(notes[0].Params), "bad key") {
		t.Errorf("want one warning about the bad key, got %+v", notes)
	}
}

// sessionWithNotes is session that also returns server notifications.
func sessionWithNotes(t *testing.T, reqs ...map[string]any) (map[string]*message, []*message) {
	t.Helper()
	var in bytes.Buffer
	for _, r := range reqs {
		r["jsonrpc"] = "2.0"
		body, _ := json.Marshal(r)
		in.WriteString("Content-Length: " + itoa(len(body)) + "\r\n\r\n")
		in.Write(body)
	}
	var out bytes.Buffer
	if err := newServer().serve(bufio.NewReader(&in), &out); err != nil {
		t.Fatalf("serve: %v", err)
	}
	replies := map[string]*message{}
	var notes []*message
	r := bufio.NewReader(&out)
	for {
		msg, err := readMessage(r)
		if err != nil {
			break
		}
		if msg.ID == nil {
			notes = append(notes, msg)
		} else {
			replies[string(msg.ID)] = msg
		}
	}
	return replies, notes
}
