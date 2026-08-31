package api

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/tastyeffectco/sandboxd/control-plane/internal/audit"
)

// File-manager operations on the app workspace, alongside the existing
// list / content / put / export endpoints in v1_files.go and
// v1_files_write.go. Like those, everything here runs on the host-side
// workspace mount — so downloads, uploads, deletes and renames work
// whether or not the sandbox is running.
//
// Security model (same paying-tenant threat model as v1_files_write.go):
//   - every caller-supplied path is lexically normalised and confined
//     under the app dir (no absolute paths, no `..` segments),
//   - every path that is opened, removed or renamed is additionally
//     checked component-by-component for symlinks (CWE-59): the tenant
//     owns the workspace and can plant links that escape it,
//   - downloads and archives stream and never follow symlinks,
//   - uploads are capped (25 MiB total, mirroring maxPutFileBytes),
//     written atomically (tmp + rename, O_NOFOLLOW tmp), never overwrite
//     a symlink leaf, and are chown'd to the workspace owner,
//   - mutations are audited.

const (
	// maxUploadBytes — total multipart body cap, mirroring maxPutFileBytes.
	maxUploadBytes = 25 << 20
)

// cleanRelPath normalises a caller-supplied app-relative path (file or
// directory) and rejects escapes. Returns the cleaned relative path —
// "." means the app dir root itself.
func cleanRelPath(raw string) (string, error) {
	if strings.ContainsRune(raw, 0) {
		return "", errors.New("invalid path: NUL byte")
	}
	if filepath.IsAbs(raw) || strings.HasPrefix(raw, "/") {
		return "", errors.New("path must be relative")
	}
	clean := filepath.Clean("/" + raw) // leading "/" so ".." can't climb out
	clean = strings.TrimPrefix(clean, "/")
	if clean == "" || clean == "." {
		return ".", nil
	}
	for _, seg := range strings.Split(clean, string(os.PathSeparator)) {
		if seg == ".." {
			return "", errors.New("path traversal (..) not allowed")
		}
	}
	return clean, nil
}

// symlinkInPath reports whether any filesystem component of full (from
// root down to, and including, the leaf unless includeLeaf is false) is a
// symlink. Missing components are not symlinks — callers that create
// them will do so safely. A true result means: refuse, the path would
// operate through a planted link.
func symlinkInPath(root, full string, includeLeaf bool) bool {
	rel, err := filepath.Rel(root, full)
	if err != nil {
		return true
	}
	if rel == "." {
		return false
	}
	parts := strings.Split(rel, string(os.PathSeparator))
	cur := root
	for i, seg := range parts {
		cur = filepath.Join(cur, seg)
		if i == len(parts)-1 && !includeLeaf {
			break
		}
		fi, err := os.Lstat(cur)
		if err != nil {
			return false // missing component: nothing to follow
		}
		if fi.Mode()&os.ModeSymlink != 0 {
			return true
		}
	}
	return false
}

// chownUnder chowns any directories under mnt that are not yet owned by
// the workspace owner — best-effort, mirroring v1PutFile.
func chownUnder(mnt, from string) {
	uid, gid := mountOwner(mnt)
	if uid < 0 {
		return
	}
	for p := from; p != mnt && strings.HasPrefix(p, mnt+string(os.PathSeparator)); p = filepath.Dir(p) {
		if fi, err := os.Stat(p); err == nil {
			if st, ok := fi.Sys().(*syscall.Stat_t); ok && (int(st.Uid) != uid || int(st.Gid) != gid) {
				_ = os.Chown(p, uid, gid)
			}
		}
	}
}

// resolveExisting resolves a caller-supplied app-relative path to a
// symlink-free on-disk path that must already exist (file or dir).
func (s *Server) resolveExisting(id, raw string) (root, full, rel string, code int, msg string) {
	if !isULID(id) {
		return "", "", "", http.StatusNotFound, "no such workspace"
	}
	root = s.appDirFor(id)
	clean, err := cleanRelPath(raw)
	if err != nil {
		return "", "", "", http.StatusBadRequest, err.Error()
	}
	full, ok := safeJoin(root, clean)
	if !ok {
		return "", "", "", http.StatusBadRequest, "invalid path"
	}
	// The leaf must exist and the whole chain must stay inside the app
	// dir even with planted symlinks (CWE-59).
	full, ok = realpathWithin(full, root)
	if !ok {
		return "", "", "", http.StatusNotFound, "no such file"
	}
	return root, full, clean, 0, ""
}

// --- GET /v1/sandboxes/{id}/files/download?path= --------------------
//
// Streams a single file as an attachment. Unlike /files/content there is
// no 2 MiB cap and the bytes are exactly what is on disk (binary-safe).

func (s *Server) v1FileDownload(w http.ResponseWriter, r *http.Request) {
	_, full, rel, code, msg := s.resolveExisting(r.PathValue("id"), r.URL.Query().Get("path"))
	if code != 0 {
		writeV1Err(w, code, "invalid_request", msg)
		return
	}
	info, err := os.Stat(full)
	if err != nil {
		writeV1Err(w, http.StatusNotFound, "not_found", "no such file")
		return
	}
	if info.IsDir() {
		writeV1Err(w, http.StatusBadRequest, "invalid_request",
			"path is a directory — use /files/archive to download it as a zip")
		return
	}
	f, err := os.Open(full)
	if err != nil {
		writeV1Err(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	defer f.Close()
	ctype := mime.TypeByExtension(strings.ToLower(filepath.Ext(full)))
	if ctype == "" {
		ctype = "application/octet-stream"
	}
	w.Header().Set("Content-Type", ctype)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": filepath.Base(rel)}))
	w.Header().Set("Content-Length", strconv.FormatInt(info.Size(), 10))
	_, _ = io.Copy(w, f)
}

// --- GET /v1/sandboxes/{id}/files/archive?path= ---------------------
//
// Streams a directory (or the whole workspace when path is omitted) as a
// zip. Mirrors v1Export: symlinks and node_modules/.git/dist/.vite are
// never included; empty directories are preserved.

func (s *Server) v1FileArchive(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	_, full, rel, code, msg := s.resolveExisting(id, r.URL.Query().Get("path"))
	if code != 0 {
		writeV1Err(w, code, "invalid_request", msg)
		return
	}
	info, err := os.Stat(full)
	if err != nil {
		writeV1Err(w, http.StatusNotFound, "not_found", "no such directory")
		return
	}
	if !info.IsDir() {
		writeV1Err(w, http.StatusBadRequest, "invalid_request",
			"path is a file — use /files/download for single files")
		return
	}
	// Name the zip after the requested directory (root → the sandbox id,
	// matching /export's behaviour).
	base := filepath.Base(rel)
	if rel == "." {
		base = id
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": base + ".zip"}))
	zw := zip.NewWriter(w)
	defer zw.Close()
	_ = filepath.WalkDir(full, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return nil // never follow/export a symlink (CWE-59)
		}
		if excludedFromFiles[d.Name()] {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		zipRel, rerr := filepath.Rel(full, path)
		if rerr != nil {
			return nil
		}
		zipPath := filepath.ToSlash(zipRel)
		if zipPath == "." {
			return nil
		}
		if d.IsDir() {
			_, _ = zw.Create(zipPath + "/") // dir entry: keeps empty dirs
			return nil
		}
		fw, werr := zw.Create(zipPath)
		if werr != nil {
			return nil
		}
		f, oerr := os.Open(path)
		if oerr != nil {
			return nil
		}
		defer f.Close()
		_, _ = io.Copy(fw, f)
		return nil
	})
	s.auditAction(r, audit.Entry{
		Action: "file.archive", Target: id,
		Detail: map[string]any{"path": rel},
	})
}

// --- POST /v1/sandboxes/{id}/files/mkdir?path= ----------------------

func (s *Server) v1FileMkdir(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !isULID(id) {
		writeV1Err(w, http.StatusNotFound, "not_found", "no such workspace")
		return
	}
	_, mnt := s.Loopback.Paths(id)
	if info, err := os.Stat(mnt); err != nil || !info.IsDir() {
		writeV1Err(w, http.StatusNotFound, "not_found", "no workspace for that sandbox")
		return
	}
	root := s.appDirFor(id)
	rel, err := cleanRelPath(r.URL.Query().Get("path"))
	if err != nil || rel == "." {
		writeV1Err(w, http.StatusBadRequest, "invalid_request", "a directory name is required")
		return
	}
	full, ok := safeJoin(root, rel)
	if !ok {
		writeV1Err(w, http.StatusBadRequest, "invalid_request", "invalid path")
		return
	}
	if _, err := os.Lstat(full); err == nil {
		writeV1Err(w, http.StatusConflict, "conflict", "a file or directory with that name already exists")
		return
	}
	if symlinkInPath(root, full, false) {
		writeV1Err(w, http.StatusBadRequest, "invalid_request", "path traverses a symlink")
		return
	}
	if err := os.MkdirAll(full, 0o775); err != nil {
		writeV1Err(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	chownUnder(mnt, full)
	s.auditAction(r, audit.Entry{
		Action: "file.mkdir", Target: id,
		Detail: map[string]any{"path": rel},
	})
	writeJSON(w, http.StatusOK, map[string]any{"path": rel})
}

// --- DELETE /v1/sandboxes/{id}/files?path= --------------------------

func (s *Server) v1FileDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !isULID(id) {
		writeV1Err(w, http.StatusNotFound, "not_found", "no such workspace")
		return
	}
	_, mnt := s.Loopback.Paths(id)
	if info, err := os.Stat(mnt); err != nil || !info.IsDir() {
		writeV1Err(w, http.StatusNotFound, "not_found", "no workspace for that sandbox")
		return
	}
	root := s.appDirFor(id)
	rel, err := cleanRelPath(r.URL.Query().Get("path"))
	if err != nil || rel == "." {
		writeV1Err(w, http.StatusBadRequest, "invalid_request", "cannot delete the workspace root")
		return
	}
	full, ok := safeJoin(root, rel)
	if !ok {
		writeV1Err(w, http.StatusBadRequest, "invalid_request", "invalid path")
		return
	}
	// Intermediate components must not be symlinks (deleting through a
	// planted link would operate outside the workspace). A symlinked
	// LEAF is fine — RemoveAll just unlinks the link itself.
	if symlinkInPath(root, full, false) {
		writeV1Err(w, http.StatusBadRequest, "invalid_request", "path traverses a symlink")
		return
	}
	if _, err := os.Lstat(full); err != nil {
		writeV1Err(w, http.StatusNotFound, "not_found", "no such file")
		return
	}
	if err := os.RemoveAll(full); err != nil {
		writeV1Err(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	s.auditAction(r, audit.Entry{
		Action: "file.delete", Target: id,
		Detail: map[string]any{"path": rel},
	})
	writeJSON(w, http.StatusOK, map[string]any{"path": rel})
}

// --- PATCH /v1/sandboxes/{id}/files?path=  {"name": "..."} ----------
//
// Rename a file or directory WITHIN its parent directory (no move).

func (s *Server) v1FileRename(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !isULID(id) {
		writeV1Err(w, http.StatusNotFound, "not_found", "no such workspace")
		return
	}
	_, mnt := s.Loopback.Paths(id)
	if info, err := os.Stat(mnt); err != nil || !info.IsDir() {
		writeV1Err(w, http.StatusNotFound, "not_found", "no workspace for that sandbox")
		return
	}
	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&body); err != nil {
		writeV1Err(w, http.StatusBadRequest, "invalid_request", "invalid JSON body")
		return
	}
	name := body.Name
	if name == "" || strings.ContainsAny(name, `/\`) || name == "." || name == ".." ||
		strings.ContainsRune(name, 0) || strings.TrimSpace(name) != name {
		writeV1Err(w, http.StatusBadRequest, "invalid_request",
			"name must be a plain file or directory name (no slashes)")
		return
	}
	root := s.appDirFor(id)
	rel, err := cleanRelPath(r.URL.Query().Get("path"))
	if err != nil || rel == "." {
		writeV1Err(w, http.StatusBadRequest, "invalid_request", "a file or directory path is required")
		return
	}
	oldFull, ok := safeJoin(root, rel)
	if !ok {
		writeV1Err(w, http.StatusBadRequest, "invalid_request", "invalid path")
		return
	}
	// Renaming THROUGH a planted symlink (leaf or intermediate) is
	// refused — the tenant could rename a link target out of the tree.
	if symlinkInPath(root, oldFull, true) {
		writeV1Err(w, http.StatusBadRequest, "invalid_request", "path traverses a symlink")
		return
	}
	if _, err := os.Lstat(oldFull); err != nil {
		writeV1Err(w, http.StatusNotFound, "not_found", "no such file")
		return
	}
	newFull := filepath.Join(filepath.Dir(oldFull), name)
	if newFull != root && !strings.HasPrefix(newFull, root+string(os.PathSeparator)) {
		writeV1Err(w, http.StatusBadRequest, "invalid_request", "invalid name")
		return
	}
	if _, err := os.Lstat(newFull); err == nil {
		writeV1Err(w, http.StatusConflict, "conflict", "a file or directory with that name already exists")
		return
	}
	if err := os.Rename(oldFull, newFull); err != nil {
		writeV1Err(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	newRel := filepath.Join(filepath.Dir(rel), name)
	s.auditAction(r, audit.Entry{
		Action: "file.rename", Target: id,
		Detail: map[string]any{"from": rel, "to": newRel},
	})
	writeJSON(w, http.StatusOK, map[string]any{"path": newRel})
}

// --- POST /v1/sandboxes/{id}/files/upload?path=<target dir> ---------
//
// Multipart upload of one or more files into the target directory. Each
// part's filename may be a RELATIVE PATH (e.g. "assets/logo.png") so the
// console can upload dropped folders in one request; parts without a
// filename are ignored. Binary-safe, streamed, never overwrites a
// symlink, atomic per file (tmp + rename), chown'd to the workspace
// owner.

func (s *Server) v1FileUpload(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !isULID(id) {
		writeV1Err(w, http.StatusNotFound, "not_found", "no such workspace")
		return
	}
	_, mnt := s.Loopback.Paths(id)
	if info, err := os.Stat(mnt); err != nil || !info.IsDir() {
		writeV1Err(w, http.StatusNotFound, "not_found", "no workspace for that sandbox")
		return
	}
	root := s.appDirFor(id)
	// Target directory ("" = app root). Must exist and be symlink-free.
	dirRel, err := cleanRelPath(r.URL.Query().Get("path"))
	if err != nil {
		writeV1Err(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	dirFull, ok := safeJoin(root, dirRel)
	if !ok {
		writeV1Err(w, http.StatusBadRequest, "invalid_request", "invalid path")
		return
	}
	dirFull, ok = realpathWithin(dirFull, root)
	if !ok {
		writeV1Err(w, http.StatusNotFound, "not_found", "no such directory")
		return
	}
	if info, err := os.Stat(dirFull); err != nil || !info.IsDir() {
		writeV1Err(w, http.StatusBadRequest, "invalid_request", "upload target is not a directory")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)
	mr, err := r.MultipartReader()
	if err != nil {
		writeV1Err(w, http.StatusBadRequest, "invalid_request", "expected multipart/form-data: "+err.Error())
		return
	}

	uid, gid := mountOwner(mnt)
	type uploaded struct {
		Path string `json:"path"`
		Size int64  `json:"size"`
	}
	var files []uploaded
	var firstErr string

	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			if firstErr == "" {
				firstErr = "read upload: " + err.Error()
			}
			break
		}
		// part.FileName() deliberately strips any directory (RFC 7578
		// §4.2), but the console uploads dropped FOLDERS by sending a
		// relative path in the filename — so parse the raw parameter
		// ourselves and validate it as a confined relative path below.
		_, params, perr := mime.ParseMediaType(part.Header.Get("Content-Disposition"))
		name := params["filename"]
		if perr != nil || name == "" {
			_, _ = io.Copy(io.Discard, part)
			part.Close()
			continue
		}
		// The filename is a relative path under the target dir. Windows
		// clients may send backslashes — normalise them first. Unlike
		// DELETE (where confined resolution is fine), any ".." segment in
		// an upload filename is REJECTED: a client that sends one
		// probably means the parent dir, and silently rewriting the name
		// would be surprising.
		normal := strings.ReplaceAll(name, "\\", "/")
		reject := false
		for _, seg := range strings.Split(normal, "/") {
			if seg == ".." {
				reject = true
				break
			}
		}
		clean, err := cleanRelPath(normal)
		if err != nil || clean == "." || reject {
			if firstErr == "" {
				if err != nil {
					firstErr = name + ": " + err.Error()
				} else {
					firstErr = name + ": invalid upload path"
				}
			}
			_, _ = io.Copy(io.Discard, part)
			part.Close()
			continue
		}
		full := filepath.Join(dirFull, filepath.FromSlash(clean))
		if full != dirFull && !strings.HasPrefix(full, dirFull+string(os.PathSeparator)) {
			if firstErr == "" {
				firstErr = name + ": escapes the target directory"
			}
			_, _ = io.Copy(io.Discard, part)
			part.Close()
			continue
		}
		// Parent chain must be symlink-free; refuse to write through a
		// planted link, and refuse to overwrite a symlink leaf.
		if symlinkInPath(root, filepath.Dir(full), true) {
			if firstErr == "" {
				firstErr = name + ": path traverses a symlink"
			}
			_, _ = io.Copy(io.Discard, part)
			part.Close()
			continue
		}
		if fi, err := os.Lstat(full); err == nil && fi.Mode()&os.ModeSymlink != 0 {
			if firstErr == "" {
				firstErr = name + ": refusing to overwrite a symlink"
			}
			_, _ = io.Copy(io.Discard, part)
			part.Close()
			continue
		}
		parent := filepath.Dir(full)
		if err := os.MkdirAll(parent, 0o775); err != nil {
			if firstErr == "" {
				firstErr = name + ": " + err.Error()
			}
			part.Close()
			continue
		}
		chownUnder(mnt, parent)

		tmp, err := os.CreateTemp(parent, ".upl-*.tmp")
		if err != nil {
			if firstErr == "" {
				firstErr = name + ": " + err.Error()
			}
			part.Close()
			continue
		}
		written, copyErr := io.Copy(tmp, part)
		closeErr := tmp.Close()
		part.Close()
		if copyErr != nil {
			_ = os.Remove(tmp.Name())
			var mbe *http.MaxBytesError
			if errors.As(copyErr, &mbe) {
				writeV1Err(w, http.StatusRequestEntityTooLarge, "invalid_request",
					"upload exceeds the 25 MiB total limit")
				return
			}
			if firstErr == "" {
				firstErr = name + ": " + copyErr.Error()
			}
			continue
		}
		if closeErr != nil {
			_ = os.Remove(tmp.Name())
			if firstErr == "" {
				firstErr = name + ": " + closeErr.Error()
			}
			continue
		}
		if uid >= 0 {
			_ = os.Chown(tmp.Name(), uid, gid)
		}
		if err := os.Chmod(tmp.Name(), 0o644); err != nil {
			_ = os.Remove(tmp.Name())
			if firstErr == "" {
				firstErr = name + ": " + err.Error()
			}
			continue
		}
		if err := os.Rename(tmp.Name(), full); err != nil {
			_ = os.Remove(tmp.Name())
			if firstErr == "" {
				firstErr = name + ": " + err.Error()
			}
			continue
		}
		files = append(files, uploaded{Path: filepath.ToSlash(filepath.Join(dirRel, filepath.FromSlash(clean))), Size: written})
	}

	if len(files) == 0 {
		if firstErr == "" {
			firstErr = "no files in upload"
		}
		writeV1Err(w, http.StatusBadRequest, "invalid_request", firstErr)
		return
	}
	s.auditAction(r, audit.Entry{
		Action: "file.upload", Target: id,
		Detail: map[string]any{"dir": dirRel, "count": len(files)},
	})
	resp := map[string]any{"dir": dirRel, "uploaded": len(files), "files": files}
	if firstErr != "" {
		resp["warning"] = firstErr
	}
	writeJSON(w, http.StatusOK, resp)
}
