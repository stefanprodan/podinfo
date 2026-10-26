package http

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func writeFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func untar(t *testing.T, data []byte) map[string]string {
	t.Helper()
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	tr := tar.NewReader(gz)
	files := map[string]string{}
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if hdr.Typeflag != tar.TypeReg || hdr.Mode != 0o644 || hdr.Uid != 0 || hdr.Gid != 0 || !hdr.ModTime.Equal(time.Unix(0, 0)) {
			t.Errorf("unexpected header for %s: %+v", hdr.Name, hdr)
		}
		b, err := io.ReadAll(tr)
		if err != nil {
			t.Fatal(err)
		}
		files[hdr.Name] = string(b)
	}
	return files
}

var testArtifactFiles = map[string]string{
	"kustomization.yaml": "resources:\n  - deployment.yaml\n",
	"deployment.yaml":    "kind: Deployment\n",
	"overlays/prod.yaml": "kind: Kustomization\n",
}

func newArtifactServer(t *testing.T, dir string) *Server {
	t.Helper()
	srv := NewMockServer()
	srv.config.ArtifactPath = dir
	srv.loadArtifact()
	srv.registerHandlers()
	return srv
}

func TestArtifactHandler(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, testArtifactFiles)
	srv := newArtifactServer(t, dir)

	rr := httptest.NewRecorder()
	srv.router.ServeHTTP(rr, httptest.NewRequest("GET", "/artifact", nil))

	if rr.Code != http.StatusOK {
		t.Fatalf("got status %d, want 200", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/gzip" {
		t.Errorf("Content-Type = %q", ct)
	}
	if rr.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Error("missing nosniff header")
	}
	body := rr.Body.Bytes()
	sum := sha256.Sum256(body)
	if want := "sha256:" + hex.EncodeToString(sum[:]); rr.Header().Get("X-Artifact-Digest") != want {
		t.Errorf("X-Artifact-Digest = %q, want %q", rr.Header().Get("X-Artifact-Digest"), want)
	}
	if rr.Header().Get("Content-Length") != strconv.Itoa(len(body)) {
		t.Errorf("Content-Length = %s, body is %d bytes", rr.Header().Get("Content-Length"), len(body))
	}

	files := untar(t, body)
	if len(files) != len(testArtifactFiles) {
		t.Fatalf("got files %v, want %v", files, testArtifactFiles)
	}
	for name, content := range testArtifactFiles {
		if files[name] != content {
			t.Errorf("%s = %q, want %q", name, files[name], content)
		}
	}
}

func TestArtifactHandler_Head(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, testArtifactFiles)
	srv := newArtifactServer(t, dir)

	get := httptest.NewRecorder()
	srv.router.ServeHTTP(get, httptest.NewRequest("GET", "/artifact", nil))
	head := httptest.NewRecorder()
	srv.router.ServeHTTP(head, httptest.NewRequest("HEAD", "/artifact", nil))

	if head.Code != http.StatusOK {
		t.Fatalf("HEAD got status %d, want 200", head.Code)
	}
	if head.Body.Len() != 0 {
		t.Errorf("HEAD returned a %d byte body", head.Body.Len())
	}
	for _, h := range []string{"Content-Type", "Content-Length", "X-Artifact-Digest"} {
		if head.Header().Get(h) == "" || head.Header().Get(h) != get.Header().Get(h) {
			t.Errorf("HEAD %s = %q, GET %s = %q", h, head.Header().Get(h), h, get.Header().Get(h))
		}
	}
}

func TestBuildArtifact_Deterministic(t *testing.T) {
	dir1 := t.TempDir()
	writeFiles(t, dir1, testArtifactFiles)

	// same files written in a different order with different mtimes
	dir2 := t.TempDir()
	for _, name := range []string{"overlays/prod.yaml", "deployment.yaml", "kustomization.yaml"} {
		writeFiles(t, dir2, map[string]string{name: testArtifactFiles[name]})
		old := time.Now().Add(-48 * time.Hour)
		if err := os.Chtimes(filepath.Join(dir2, name), old, old); err != nil {
			t.Fatal(err)
		}
	}

	a1, err := buildArtifact(dir1)
	if err != nil {
		t.Fatal(err)
	}
	a1again, err := buildArtifact(dir1)
	if err != nil {
		t.Fatal(err)
	}
	a2, err := buildArtifact(dir2)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(a1.data, a1again.data) {
		t.Error("two builds of the same dir differ")
	}
	if a1.digest != a2.digest {
		t.Errorf("file order or mtime changed the digest: %s != %s", a1.digest, a2.digest)
	}
}

func TestBuildArtifact_SkipsSymlinks(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"deployment.yaml": "kind: Deployment\n"})
	outside := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(outside, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "link.yaml")); err != nil {
		t.Skipf("symlinks not supported: %v", err)
	}

	a, err := buildArtifact(dir)
	if err != nil {
		t.Fatal(err)
	}
	files := untar(t, a.data)
	if _, ok := files["link.yaml"]; ok {
		t.Error("symlink was included in the artifact")
	}
	if len(files) != 1 {
		t.Errorf("got files %v, want only deployment.yaml", files)
	}
}

func TestArtifactHandler_MissingDir(t *testing.T) {
	srv := newArtifactServer(t, filepath.Join(t.TempDir(), "missing"))

	rr := httptest.NewRecorder()
	srv.router.ServeHTTP(rr, httptest.NewRequest("GET", "/artifact", nil))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("got status %d, want 404", rr.Code)
	}
}

func TestArtifactHandler_Disabled(t *testing.T) {
	srv := newArtifactServer(t, "")

	rr := httptest.NewRecorder()
	srv.router.ServeHTTP(rr, httptest.NewRequest("GET", "/artifact", nil))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("got status %d, want 404", rr.Code)
	}
	if srv.artifact != nil {
		t.Fatal("artifact built with an empty --artifact-path")
	}
}

func TestArtifactHandler_Prefix(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, testArtifactFiles)
	srv := NewMockServer()
	srv.config.Prefix = normalizePrefix("/foo")
	srv.config.ArtifactPath = dir
	srv.loadArtifact()
	srv.registerHandlers()

	rr := httptest.NewRecorder()
	srv.router.ServeHTTP(rr, httptest.NewRequest("GET", "/foo/artifact", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /foo/artifact got status %d, want 200", rr.Code)
	}
}
