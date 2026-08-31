package api

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// --- helpers ---------------------------------------------------------

func callFiles(t *testing.T, s *Server, id, method, rawURL string, body io.Reader, contentType string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, rawURL, body)
	if contentType != "" {
		r.Header.Set("Content-Type", contentType)
	}
	r.SetPathValue("id", id)
	w := httptest.NewRecorder()
	switch method {
	case "GET":
		if strings.Contains(rawURL, "/download") {
			s.v1FileDownload(w, r)
		} else if strings.Contains(rawURL, "/archive") {
			s.v1FileArchive(w, r)
		} else if strings.Contains(rawURL, "/files/content") {
			s.v1FileContent(w, r)
		} else {
			s.v1ListFiles(w, r)
		}
	case "POST":
		if strings.Contains(rawURL, "/upload") {
			s.v1FileUpload(w, r)
		} else {
			s.v1FileMkdir(w, r)
		}
	case "DELETE":
		s.v1FileDelete(w, r)
	case "PATCH":
		s.v1FileRename(w, r)
	}
	return w
}

func filesURL(id, suffix, path string) string {
	return "/v1/sandboxes/" + id + suffix + "?path=" + url.QueryEscape(path)
}

func uploadBody(t *testing.T, files map[string]string) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for name, content := range files {
		fw, err := mw.CreateFormFile("files", name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	return &buf, mw.FormDataContentType()
}

func appFile(t *testing.T, s *Server, id, rel, content string) string {
	t.Helper()
	_, mnt := s.Loopback.Paths(id)
	p := filepath.Join(mnt, appSubdir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// --- download --------------------------------------------------------

func TestFileDownload(t *testing.T) {
	s, id := fileSymlinkServer(t)

	w := callFiles(t, s, id, "GET", filesURL(id, "/files/download", "ok.txt"), nil, "")
	if w.Code != 200 {
		t.Fatalf("download ok.txt: got %d", w.Code)
	}
	if w.Body.String() != "hello" {
		t.Fatalf("body = %q, want hello", w.Body.String())
	}
	if cd := w.Header().Get("Content-Disposition"); !strings.Contains(cd, "filename=ok.txt") {
		t.Fatalf("Content-Disposition = %q", cd)
	}

	// A directory is not downloadable — point at the archive endpoint.
	if w := callFiles(t, s, id, "GET", filesURL(id, "/files/download", "."), nil, ""); w.Code != 400 {
		t.Fatalf("download dir: got %d, want 400", w.Code)
	}
	// Missing file.
	if w := callFiles(t, s, id, "GET", filesURL(id, "/files/download", "nope.txt"), nil, ""); w.Code != 404 {
		t.Fatalf("download missing: got %d, want 404", w.Code)
	}
	// A leaf symlink escaping the workspace must never resolve.
	if w := callFiles(t, s, id, "GET", filesURL(id, "/files/download", "leak"), nil, ""); w.Code != 404 {
		t.Fatalf("download leaf symlink: got %d, want 404", w.Code)
	}
	// An intermediate symlink escaping the workspace must never resolve.
	if w := callFiles(t, s, id, "GET", filesURL(id, "/files/download", "escape/environ"), nil, ""); w.Code != 404 {
		t.Fatalf("download intermediate symlink: got %d, want 404", w.Code)
	}
}

// --- archive ---------------------------------------------------------

func TestFileArchive(t *testing.T) {
	s, id := fileSymlinkServer(t)
	appFile(t, s, id, "sub/a.txt", "AAA")
	appFile(t, s, id, "sub/node_modules/x.js", "excluded")
	if err := os.MkdirAll(filepath.Join(appDirOf(t, s, id), "sub/empty"), 0o755); err != nil {
		t.Fatal(err)
	}

	w := callFiles(t, s, id, "GET", filesURL(id, "/files/archive", "sub"), nil, "")
	if w.Code != 200 {
		t.Fatalf("archive sub: got %d body=%s", w.Code, w.Body.String())
	}
	if cd := w.Header().Get("Content-Disposition"); !strings.Contains(cd, "filename=sub.zip") {
		t.Fatalf("Content-Disposition = %q", cd)
	}
	zr, err := zip.NewReader(bytes.NewReader(w.Body.Bytes()), int64(w.Body.Len()))
	if err != nil {
		t.Fatalf("not a valid zip: %v", err)
	}
	got := map[string]bool{}
	for _, f := range zr.File {
		got[f.Name] = true
	}
	if !got["a.txt"] {
		t.Errorf("zip missing a.txt: %v", got)
	}
	if !got["empty/"] {
		t.Errorf("zip missing empty dir entry: %v", got)
	}
	for name := range got {
		if strings.Contains(name, "node_modules") {
			t.Errorf("zip includes excluded node_modules: %v", got)
		}
	}

	// Archiving a file is a caller error (use /download).
	if w := callFiles(t, s, id, "GET", filesURL(id, "/files/archive", "ok.txt"), nil, ""); w.Code != 400 {
		t.Fatalf("archive file: got %d, want 400", w.Code)
	}
	// A symlinked dir escaping the workspace must never be zipped.
	if w := callFiles(t, s, id, "GET", filesURL(id, "/files/archive", "escape"), nil, ""); w.Code != 404 {
		t.Fatalf("archive symlink dir: got %d, want 404", w.Code)
	}
}

// --- mkdir -----------------------------------------------------------

func TestFileMkdir(t *testing.T) {
	s, id := fileSymlinkServer(t)

	if w := callFiles(t, s, id, "POST", filesURL(id, "/files/mkdir", "assets/img"), nil, ""); w.Code != 200 {
		t.Fatalf("mkdir: got %d body=%s", w.Code, w.Body.String())
	}
	if fi, err := os.Stat(filepath.Join(appDirOf(t, s, id), "assets/img")); err != nil || !fi.IsDir() {
		t.Fatalf("assets/img not created: %v", err)
	}
	// Exists → conflict.
	if w := callFiles(t, s, id, "POST", filesURL(id, "/files/mkdir", "assets"), nil, ""); w.Code != 409 {
		t.Fatalf("mkdir existing: got %d, want 409", w.Code)
	}
	// Root itself is not creatable.
	if w := callFiles(t, s, id, "POST", filesURL(id, "/files/mkdir", "."), nil, ""); w.Code != 400 {
		t.Fatalf("mkdir root: got %d, want 400", w.Code)
	}
	// Through a planted symlink is refused.
	if w := callFiles(t, s, id, "POST", filesURL(id, "/files/mkdir", "escape/x"), nil, ""); w.Code != 400 {
		t.Fatalf("mkdir through symlink: got %d, want 400", w.Code)
	}
}

// --- delete ----------------------------------------------------------

func TestFileDelete(t *testing.T) {
	s, id := fileSymlinkServer(t)
	_, mnt := s.Loopback.Paths(id)
	appDir := filepath.Join(mnt, appSubdir)
	appFile(t, s, id, "dir/deep/f.txt", "x")

	// Recursive delete of a directory.
	if w := callFiles(t, s, id, "DELETE", filesURL(id, "/files", "dir"), nil, ""); w.Code != 200 {
		t.Fatalf("delete dir: got %d body=%s", w.Code, w.Body.String())
	}
	if _, err := os.Stat(filepath.Join(appDir, "dir")); !os.IsNotExist(err) {
		t.Fatalf("dir still exists: %v", err)
	}

	// Root is protected.
	if w := callFiles(t, s, id, "DELETE", filesURL(id, "/files", "."), nil, ""); w.Code != 400 {
		t.Fatalf("delete root: got %d, want 400", w.Code)
	}
	// Deleting through an intermediate symlink is refused.
	if w := callFiles(t, s, id, "DELETE", filesURL(id, "/files", "escape/environ"), nil, ""); w.Code != 400 {
		t.Fatalf("delete through symlink: got %d, want 400", w.Code)
	}

	// Traversal resolves CONFINED: "../escape" → "escape" (the planted
	// link inside the app dir) — the link is unlinked, the OUTSIDE
	// directory it pointed at is untouched.
	escapeTarget := readLinkTarget(t, filepath.Join(appDir, "escape"))
	if w := callFiles(t, s, id, "DELETE", filesURL(id, "/files", "../escape"), nil, ""); w.Code != 200 {
		t.Fatalf("delete ../escape: got %d body=%s, want 200", w.Code, w.Body.String())
	}
	if _, err := os.Lstat(filepath.Join(appDir, "escape")); !os.IsNotExist(err) {
		t.Fatalf("confined link still present")
	}
	if fi, err := os.Stat(escapeTarget); err != nil || !fi.IsDir() {
		t.Fatalf("symlink target outside the workspace was affected: %v", err)
	}

	// Deleting a symlink LEAF unlinks the link, never the target.
	leakTarget := readLinkTarget(t, filepath.Join(appDir, "leak"))
	if w := callFiles(t, s, id, "DELETE", filesURL(id, "/files", "leak"), nil, ""); w.Code != 200 {
		t.Fatalf("delete symlink leaf: got %d", w.Code)
	}
	if _, err := os.Lstat(filepath.Join(appDir, "leak")); !os.IsNotExist(err) {
		t.Fatalf("leak still present")
	}
	if b, err := os.ReadFile(leakTarget); err != nil || !strings.Contains(string(b), secretMarker) {
		t.Fatalf("symlink target content changed: %v", err)
	}
}

func readLinkTarget(t *testing.T, link string) string {
	t.Helper()
	dst, err := os.Readlink(link)
	if err != nil {
		t.Fatal(err)
	}
	return dst
}

// --- rename ----------------------------------------------------------

func TestFileRename(t *testing.T) {
	s, id := fileSymlinkServer(t)
	renameBody := func(name string) io.Reader { return strings.NewReader(`{"name":` + strconv.Quote(name) + `}`) }

	// Happy path: rename a file in place.
	w := callFiles(t, s, id, "PATCH", filesURL(id, "/files", "ok.txt"), renameBody("greeting.txt"), "application/json")
	if w.Code != 200 {
		t.Fatalf("rename: got %d body=%s", w.Code, w.Body.String())
	}
	oldP := filepath.Join(appDirOf(t, s, id), "ok.txt")
	newP := filepath.Join(appDirOf(t, s, id), "greeting.txt")
	if _, err := os.Stat(oldP); !os.IsNotExist(err) {
		t.Fatalf("old file still exists")
	}
	if b, err := os.ReadFile(newP); err != nil || string(b) != "hello" {
		t.Fatalf("renamed file content: %v", err)
	}

	// Rename a directory.
	appFile(t, s, id, "subdir/f.txt", "x")
	if w := callFiles(t, s, id, "PATCH", filesURL(id, "/files", "subdir"), renameBody("renamed_dir"), "application/json"); w.Code != 200 {
		t.Fatalf("rename dir: got %d body=%s", w.Code, w.Body.String())
	}

	// Target exists → conflict.
	if w := callFiles(t, s, id, "PATCH", filesURL(id, "/files", "greeting.txt"), renameBody("renamed_dir"), "application/json"); w.Code != 409 {
		t.Fatalf("rename conflict: got %d, want 409", w.Code)
	}
	// Slash in name → 400.
	if w := callFiles(t, s, id, "PATCH", filesURL(id, "/files", "greeting.txt"), renameBody("a/b"), "application/json"); w.Code != 400 {
		t.Fatalf("rename with slash: got %d, want 400", w.Code)
	}
	// Missing source → 404.
	if w := callFiles(t, s, id, "PATCH", filesURL(id, "/files", "gone.txt"), renameBody("x"), "application/json"); w.Code != 404 {
		t.Fatalf("rename missing: got %d, want 404", w.Code)
	}
	// Renaming a symlink leaf is refused.
	if w := callFiles(t, s, id, "PATCH", filesURL(id, "/files", "leak"), renameBody("leak2"), "application/json"); w.Code != 400 {
		t.Fatalf("rename symlink: got %d, want 400", w.Code)
	}
}

// --- upload ----------------------------------------------------------

func TestFileUpload(t *testing.T) {
	s, id := fileSymlinkServer(t)

	body, ct := uploadBody(t, map[string]string{
		"one.txt":      "ONE",
		"nested/two.md": "TWO",
	})
	w := callFiles(t, s, id, "POST", filesURL(id, "/files/upload", ""), body, ct)
	if w.Code != 200 {
		t.Fatalf("upload: got %d body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Uploaded int `json:"uploaded"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Uploaded != 2 {
		t.Fatalf("uploaded = %d, want 2 (%s)", resp.Uploaded, w.Body.String())
	}
	if b, err := os.ReadFile(filepath.Join(appDirOf(t, s, id), "one.txt")); err != nil || string(b) != "ONE" {
		t.Fatalf("one.txt: %v", err)
	}
	if b, err := os.ReadFile(filepath.Join(appDirOf(t, s, id), "nested/two.md")); err != nil || string(b) != "TWO" {
		t.Fatalf("nested/two.md: %v", err)
	}

	// Traversal in a part filename is refused; other parts still land.
	body, ct = uploadBody(t, map[string]string{
		"three.txt":   "THREE",
		"../evil.txt": "EVIL",
	})
	w = callFiles(t, s, id, "POST", filesURL(id, "/files/upload", ""), body, ct)
	if w.Code != 200 {
		t.Fatalf("upload with traversal part: got %d body=%s", w.Code, w.Body.String())
	}
	if _, err := os.Stat(filepath.Join(appDirOf(t, s, id), "three.txt")); err != nil {
		t.Fatalf("three.txt not written: %v", err)
	}
	if _, err := os.Stat(filepath.Join(appDirOf(t, s, id), "evil.txt")); !os.IsNotExist(err) {
		t.Fatalf("evil.txt escaped the target dir")
	}

	// Uploading ONTO a symlink leaf is refused.
	if err := os.Symlink(filepath.Join(appDirOf(t, s, id), "one.txt"), filepath.Join(appDirOf(t, s, id), "link.txt")); err != nil {
		t.Fatal(err)
	}
	body, ct = uploadBody(t, map[string]string{"link.txt": "HIJACK"})
	if w = callFiles(t, s, id, "POST", filesURL(id, "/files/upload", ""), body, ct); w.Code != 400 {
		t.Fatalf("upload onto symlink: got %d body=%s, want 400", w.Code, w.Body.String())
	}
	if b, _ := os.ReadFile(filepath.Join(appDirOf(t, s, id), "one.txt")); string(b) != "ONE" {
		t.Fatalf("symlink target overwritten")
	}

	// Upload into a missing directory → 404.
	body, ct = uploadBody(t, map[string]string{"x.txt": "x"})
	if w = callFiles(t, s, id, "POST", filesURL(id, "/files/upload", "no/such/dir"), body, ct); w.Code != 404 {
		t.Fatalf("upload missing dir: got %d, want 404", w.Code)
	}
}

// --- content types (image preview) ------------------------------------

func TestFileContentImageTypes(t *testing.T) {
	s, id := fileSymlinkServer(t)
	png := append([]byte{0x89, 'P', 'N', 'G'}, make([]byte, 16)...)
	appFile(t, s, id, "logo.png", string(png))
	appFile(t, s, id, "icon.svg", `<svg xmlns="http://www.w3.org/2000/svg"></svg>`)

	w := callFiles(t, s, id, "GET", filesURL(id, "/files/content", "logo.png"), nil, "")
	if w.Code != 200 || w.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("png: got %d %q", w.Code, w.Header().Get("Content-Type"))
	}
	if w.Header().Get("Content-Security-Policy") != "sandbox" {
		t.Fatalf("png missing sandbox CSP")
	}
	w = callFiles(t, s, id, "GET", filesURL(id, "/files/content", "icon.svg"), nil, "")
	if w.Header().Get("Content-Type") != "image/svg+xml" {
		t.Fatalf("svg: %q", w.Header().Get("Content-Type"))
	}
	w = callFiles(t, s, id, "GET", filesURL(id, "/files/content", "ok.txt"), nil, "")
	if w.Header().Get("Content-Type") != "text/plain; charset=utf-8" {
		t.Fatalf("txt: %q", w.Header().Get("Content-Type"))
	}
}

// --- path validation ---------------------------------------------------

func TestCleanRelPath(t *testing.T) {
	cases := []struct {
		raw   string
		want  string
	 scary bool
	}{
		{"a.txt", "a.txt", false},
		{"assets/img/logo.png", "assets/img/logo.png", false},
		{"./a.txt", "a.txt", false},
		{"", ".", false},           // root — callers decide if allowed
		{".", ".", false},          // root
		{"a//b", "a/b", false},     // collapsed
		{"../x", "x", false},       // confined, like safeJoin: ../ climbs to the root and stops
		{"a/../../x", "x", false},  // confined the same way
		{"/etc/passwd", "", true},  {"a\x00b", "", true},
	}
	for _, tc := range cases {
		got, err := cleanRelPath(tc.raw)
		if tc.scary {
			if err == nil {
				t.Errorf("cleanRelPath(%q) = %q, want error", tc.raw, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("cleanRelPath(%q): %v", tc.raw, err)
			continue
		}
		if got != tc.want {
			t.Errorf("cleanRelPath(%q) = %q, want %q", tc.raw, got, tc.want)
		}
	}
}

func TestSymlinkInPath(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "a/b"), 0o755); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "a/link")); err != nil {
		t.Fatal(err)
	}
	if !symlinkInPath(root, filepath.Join(root, "a/link/x"), false) {
		t.Errorf("intermediate symlink not detected")
	}
	if !symlinkInPath(root, filepath.Join(root, "a/link"), true) {
		t.Errorf("leaf symlink not detected")
	}
	if symlinkInPath(root, filepath.Join(root, "a/link"), false) {
		t.Errorf("leaf-only check flagged a leaf symlink it should ignore")
	}
	if symlinkInPath(root, filepath.Join(root, "a/b/new"), false) {
		t.Errorf("clean path flagged")
	}
	// Missing components are not symlinks.
	if symlinkInPath(root, filepath.Join(root, "a/zz/y"), false) {
		t.Errorf("missing intermediate flagged as symlink")
	}
}

// appDirOf returns the on-disk app dir for a test sandbox.
func appDirOf(t *testing.T, s *Server, id string) string {
	t.Helper()
	_, mnt := s.Loopback.Paths(id)
	return filepath.Join(mnt, appSubdir)
}
