package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"log"
	"strings"

	"go-tags-ls/catalog"
)

// LSP protocol subset used by Phase 0.

type position struct {
	Line      int `json:"line"`
	Character int `json:"character"` // UTF-16 code units
}

type textDocumentItem struct {
	URI  string `json:"uri"`
	Text string `json:"text"`
}

type didOpenParams struct {
	TextDocument textDocumentItem `json:"textDocument"`
}

type didChangeParams struct {
	TextDocument   struct{ URI string } `json:"textDocument"`
	ContentChanges []struct {
		Text string `json:"text"`
	} `json:"contentChanges"`
}

type didCloseParams struct {
	TextDocument struct{ URI string } `json:"textDocument"`
}

type completionParams struct {
	TextDocument struct {
		URI string `json:"uri"`
	} `json:"textDocument"`
	Position position `json:"position"`
}

const textDocumentSyncFull = 1

// triggerCharacters open the completion popup automatically inside a Struct Tag.
var triggerCharacters = []string{"`", `"`, ",", ";", "|", ":", "=", " "}

type server struct {
	docs       map[string]string // URI -> full text
	keys       []catalog.Key     // built-in keys with the user's settings applied
	typeSearch string            // typeSearchPackage or typeSearchFile
	problems   []string          // ignored settings, reported after initialize
	fileCache  map[string]cachedFile
	out        io.Writer
	shutdown   bool
}

func newServer() *server {
	return &server{
		docs:       map[string]string{},
		keys:       catalog.Keys,
		typeSearch: typeSearchPackage,
		fileCache:  map[string]cachedFile{},
	}
}

type initializeParams struct {
	InitializationOptions json.RawMessage `json:"initializationOptions"`
}

// configure applies the user's settings from initializationOptions.
func (s *server) configure(raw json.RawMessage) {
	if len(raw) == 0 || string(raw) == "null" {
		return
	}
	var cfg catalog.Config
	if err := json.Unmarshal(raw, &cfg); err != nil {
		s.problems = append(s.problems, "settings ignored: "+err.Error())
		return
	}
	s.keys, s.problems = catalog.Configure(cfg)
	switch cfg.TypeSearch {
	case "", typeSearchPackage:
	case typeSearchFile:
		s.typeSearch = typeSearchFile
	default:
		s.problems = append(s.problems, `type_search: unknown value "`+cfg.TypeSearch+`" (use "package" or "file")`)
	}
}

// notify sends a server-to-client notification.
func (s *server) notify(method string, params any) {
	raw, err := json.Marshal(params)
	if err == nil {
		err = writeMessage(s.out, &message{Method: method, Params: raw})
	}
	if err != nil {
		log.Printf("notify %s: %v", method, err)
	}
}

var errExit = errors.New("exit")

func (s *server) serve(r *bufio.Reader, w io.Writer) error {
	s.out = w
	for {
		msg, err := readMessage(r)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		result, rerr, err := s.handle(msg)
		if errors.Is(err, errExit) {
			return nil
		}
		if err != nil {
			log.Printf("%s: %v", msg.Method, err)
		}
		if msg.ID == nil { // notification: no reply
			continue
		}
		reply := &message{ID: msg.ID, Result: result, Error: rerr}
		if rerr == nil && result == nil {
			reply.Result = json.RawMessage("null")
		}
		if err := writeMessage(w, reply); err != nil {
			return err
		}
	}
}

func (s *server) handle(msg *message) (any, *rpcError, error) {
	switch msg.Method {
	case "initialize":
		var p initializeParams
		if len(msg.Params) > 0 {
			if err := json.Unmarshal(msg.Params, &p); err != nil {
				return nil, &rpcError{Code: -32602, Message: err.Error()}, nil
			}
		}
		s.configure(p.InitializationOptions)
		return map[string]any{
			"capabilities": map[string]any{
				"textDocumentSync": textDocumentSyncFull,
				"completionProvider": map[string]any{
					"triggerCharacters": triggerCharacters,
				},
			},
			"serverInfo": map[string]any{"name": "go-tags-ls", "version": version},
		}, nil, nil

	case "initialized":
		for _, problem := range s.problems {
			log.Print(problem)
			s.notify("window/showMessage", map[string]any{
				"type":    2, // Warning
				"message": "Go Tags: " + problem,
			})
		}
		return nil, nil, nil

	case "shutdown":
		s.shutdown = true
		return nil, nil, nil

	case "exit":
		return nil, nil, errExit

	case "textDocument/didOpen":
		var p didOpenParams
		if err := json.Unmarshal(msg.Params, &p); err != nil {
			return nil, nil, err
		}
		s.docs[p.TextDocument.URI] = p.TextDocument.Text
		return nil, nil, nil

	case "textDocument/didChange":
		var p didChangeParams
		if err := json.Unmarshal(msg.Params, &p); err != nil {
			return nil, nil, err
		}
		if n := len(p.ContentChanges); n > 0 { // full sync: last change is whole text
			s.docs[p.TextDocument.URI] = p.ContentChanges[n-1].Text
		}
		return nil, nil, nil

	case "textDocument/didClose":
		var p didCloseParams
		if err := json.Unmarshal(msg.Params, &p); err != nil {
			return nil, nil, err
		}
		delete(s.docs, p.TextDocument.URI)
		return nil, nil, nil

	case "textDocument/completion":
		var p completionParams
		if err := json.Unmarshal(msg.Params, &p); err != nil {
			return nil, &rpcError{Code: -32602, Message: err.Error()}, nil
		}
		return s.complete(p), nil, nil
	}

	if msg.ID != nil {
		return nil, &rpcError{Code: -32601, Message: "method not found: " + msg.Method}, nil
	}
	return nil, nil, nil // unknown notification: ignore
}

// byteColumn converts an LSP position (UTF-16 units) to a byte column on its line.
func byteColumn(text string, pos position) (int, bool) {
	lines := strings.Split(text, "\n")
	if pos.Line < 0 || pos.Line >= len(lines) {
		return 0, false
	}
	line := strings.TrimSuffix(lines[pos.Line], "\r")
	units := 0
	for i, r := range line {
		if units >= pos.Character {
			return i, true
		}
		if r >= 0x10000 {
			units += 2
		} else {
			units++
		}
	}
	return len(line), true
}
