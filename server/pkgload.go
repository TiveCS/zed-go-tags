package main

import (
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"go-tags-ls/pkgindex"
)

// Type search modes (setting "type_search").
const (
	typeSearchPackage = "package" // every .go file in the current file's folder
	typeSearchFile    = "file"    // only the current file
)

type cachedFile struct {
	mod  time.Time
	size int64
	file *pkgindex.File
}

// packageIndex indexes the package of the document at uri. Open documents
// are read from their live text, other files from disk (cached until their
// modification time or size changes).
func (s *server) packageIndex(uri, text string) *pkgindex.Index {
	cur := pkgindex.ParseFile(text)
	files := []*pkgindex.File{cur}
	path, ok := uriToPath(uri)
	if s.typeSearch == typeSearchFile || !ok {
		return pkgindex.Merge(cur.Package, files)
	}
	dir := filepath.Dir(path)

	open := map[string]string{} // path -> live text, for other open files in dir
	for u, t := range s.docs {
		if p, ok := uriToPath(u); ok && samePath(filepath.Dir(p), dir) && !samePath(p, path) {
			open[pathKey(p)] = t
		}
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return pkgindex.Merge(cur.Package, files)
	}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") {
			continue
		}
		full := filepath.Join(dir, name)
		if samePath(full, path) {
			continue
		}
		if t, ok := open[pathKey(full)]; ok {
			files = append(files, pkgindex.ParseFile(t))
			continue
		}
		if f := s.diskFile(full); f != nil {
			files = append(files, f)
		}
	}
	return pkgindex.Merge(cur.Package, files)
}

func (s *server) diskFile(path string) *pkgindex.File {
	info, err := os.Stat(path)
	if err != nil {
		return nil
	}
	key := pathKey(path)
	if c, ok := s.fileCache[key]; ok && c.mod.Equal(info.ModTime()) && c.size == info.Size() {
		return c.file
	}
	src, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	f := pkgindex.ParseFile(string(src))
	s.fileCache[key] = cachedFile{mod: info.ModTime(), size: info.Size(), file: f}
	return f
}

// uriToPath converts a file:// URI to a local path.
func uriToPath(uri string) (string, bool) {
	u, err := url.Parse(uri)
	if err != nil || u.Scheme != "file" {
		return "", false
	}
	p := u.Path
	if runtime.GOOS == "windows" && len(p) >= 3 && p[0] == '/' && p[2] == ':' {
		p = p[1:] // /E:/codes/x.go -> E:/codes/x.go
	}
	return filepath.Clean(filepath.FromSlash(p)), true
}

// pathKey normalises a path for comparison (Windows paths are case-insensitive).
func pathKey(p string) string {
	p = filepath.Clean(p)
	if runtime.GOOS == "windows" {
		p = strings.ToLower(p)
	}
	return p
}

func samePath(a, b string) bool { return pathKey(a) == pathKey(b) }
