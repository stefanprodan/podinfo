// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: Apache-2.0

package store

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func newTestStore(t *testing.T, dir string) (*Store, *tracetest.InMemoryExporter) {
	t.Helper()
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })
	return New(dir, tp.Tracer("test")), exporter
}

func assertSpan(t *testing.T, exporter *tracetest.InMemoryExporter, name, file string, code codes.Code) {
	t.Helper()
	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("got %d spans, want 1", len(spans))
	}
	span := spans[0]
	if span.Name != name || span.Status.Code != code {
		t.Errorf("span %q %v, want %q %v", span.Name, span.Status.Code, name, code)
	}
	if code == codes.Error && len(span.Events) != 1 {
		t.Errorf("got %d span events, want the recorded error", len(span.Events))
	}
	want := attribute.String("file.path", file)
	if slices.Contains(span.Attributes, want) {
		return
	}
	t.Errorf("span attributes %v, want %v", span.Attributes, want)
}

func TestHash(t *testing.T) {
	if got, want := Hash([]byte("hello world")), "2aae6c35c94fcfb415dbe95f408b9ce91ee846ed"; got != want {
		t.Errorf("Hash = %q, want %q", got, want)
	}
}

func TestHashPattern(t *testing.T) {
	for hash, want := range map[string]bool{
		strings.Repeat("a", 40): true,
		strings.Repeat("A", 40): false,
		strings.Repeat("a", 39): false,
		strings.Repeat("g", 40): false,
		"../etc/passwd":         false,
		"":                      false,
	} {
		if got := hashPattern.MatchString(hash); got != want {
			t.Errorf("hashPattern.MatchString(%q) = %v, want %v", hash, got, want)
		}
	}
}

func TestWriteRead(t *testing.T) {
	dir := t.TempDir()
	s, exporter := newTestStore(t, dir)
	content := []byte{0xff, 0xfe, 'd', 'a', 't', 'a'}

	hash, err := s.Write(context.Background(), content)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if hash != Hash(content) {
		t.Errorf("Write returned %q, want %q", hash, Hash(content))
	}
	file := filepath.Join(dir, hash)
	assertSpan(t, exporter, "store write", file, codes.Unset)
	info, err := os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}
	if mode := info.Mode().Perm(); mode != 0o644 {
		t.Errorf("file mode %v, want %v", mode, os.FileMode(0o644))
	}
	exporter.Reset()

	got, err := s.Read(context.Background(), hash)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Errorf("Read returned %q, want %q", got, content)
	}
	assertSpan(t, exporter, "store read", file, codes.Unset)
}

func TestWriteConcurrentRead(t *testing.T) {
	dir := t.TempDir()
	s, _ := newTestStore(t, dir)
	content := bytes.Repeat([]byte("podinfo "), 1<<20)
	hash, err := s.Write(context.Background(), content)
	if err != nil {
		t.Fatal(err)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		for range 20 {
			if _, err := s.Write(context.Background(), content); err != nil {
				t.Errorf("Write: %v", err)
				return
			}
		}
	}()
	for running := true; running; {
		select {
		case <-done:
			running = false
		default:
		}
		got, err := s.Read(context.Background(), hash)
		if err != nil {
			t.Fatalf("Read: %v", err)
		}
		if len(got) != len(content) {
			t.Fatalf("Read returned %d bytes during a write, want %d", len(got), len(content))
		}
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != hash {
		t.Errorf("store holds %v, want only %s", entries, hash)
	}
}

func TestSweep(t *testing.T) {
	dir := t.TempDir()
	old := time.Now().Add(-time.Hour)
	hash := Hash([]byte("data"))
	for name, mtime := range map[string]time.Time{
		".tmp-stale": old,
		".tmp-fresh": time.Now(),
		hash:         old,
		"other":      old,
	} {
		file := filepath.Join(dir, name)
		if err := os.WriteFile(file, []byte("data"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(file, mtime, mtime); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, ".tmp-dir"), 0o750); err != nil {
		t.Fatal(err)
	}

	if err := Sweep(dir); err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	got := make([]string, 0, len(entries))
	for _, entry := range entries {
		got = append(got, entry.Name())
	}
	want := []string{".tmp-dir", ".tmp-fresh", hash, "other"}
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("store holds %v, want %v", got, want)
	}

	if err := Sweep(filepath.Join(dir, "missing")); err != nil {
		t.Errorf("Sweep of a missing directory: %v", err)
	}
}

func TestWriteError(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "missing")
	s, exporter := newTestStore(t, dir)

	hash, err := s.Write(context.Background(), []byte("data"))
	if err == nil || hash != "" {
		t.Fatalf("Write returned %q, %v, want an error", hash, err)
	}
	assertSpan(t, exporter, "store write", filepath.Join(dir, Hash([]byte("data"))), codes.Error)
}

func TestReadErrors(t *testing.T) {
	dir := t.TempDir()
	isDir := strings.Repeat("b", 40)
	if err := os.Mkdir(filepath.Join(dir, isDir), 0o750); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		hash string
		want func(error) bool
		span bool
	}{
		{name: "invalid hash", hash: "../etc/passwd", want: func(err error) bool { return errors.Is(err, ErrInvalidHash) }},
		{name: "missing file", hash: strings.Repeat("a", 40), want: func(err error) bool { return errors.Is(err, fs.ErrNotExist) }, span: true},
		{name: "directory", hash: isDir, want: func(err error) bool {
			return err != nil && !errors.Is(err, fs.ErrNotExist) && !errors.Is(err, ErrInvalidHash)
		}, span: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, exporter := newTestStore(t, dir)
			content, err := s.Read(context.Background(), tt.hash)
			if !tt.want(err) || content != nil {
				t.Fatalf("Read returned %q, %v", content, err)
			}
			if !tt.span {
				if spans := exporter.GetSpans(); len(spans) != 0 {
					t.Errorf("got %d spans, want none", len(spans))
				}
				return
			}
			assertSpan(t, exporter, "store read", filepath.Join(dir, tt.hash), codes.Error)
		})
	}
}
