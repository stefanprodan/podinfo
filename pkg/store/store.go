// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: Apache-2.0

package store

import (
	"context"
	"crypto/rand"
	"crypto/sha1" //nolint:gosec // SHA1 names the stored files, it is not a security control
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/trace"
)

// ErrInvalidHash is returned by Read for a hash that is not a lowercase SHA1 hex digest.
var ErrInvalidHash = errors.New("invalid hash")

var hashPattern = regexp.MustCompile(`^[a-f0-9]{40}$`)

// Hash returns the SHA1 hex digest of content, which names its file.
func Hash(content []byte) string {
	h := sha1.Sum(content) //nolint:gosec // SHA1 names the stored files, it is not a security control
	return hex.EncodeToString(h[:])
}

// Store writes and reads the files in dir, each in a "store write" or
// "store read" span.
type Store struct {
	dir    string
	tracer trace.Tracer
}

// New returns a Store for the files in dir that starts its spans with tracer.
func New(dir string, tracer trace.Tracer) *Store {
	return &Store{dir: dir, tracer: tracer}
}

// Write saves content to its file and returns its hash.
func (s *Store) Write(ctx context.Context, content []byte) (string, error) {
	hash := Hash(content)
	err := s.do(ctx, "store write", hash, func(file string) error {
		return writeFile(file, content)
	})
	if err != nil {
		return "", err
	}
	return hash, nil
}

const tempPrefix = ".tmp-"

// Sweep removes the temporary files left in dir by writes that were
// interrupted. Files modified in the last minute are kept, another process
// sharing dir may still be writing them.
func Sweep(dir string) error {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var errs []error
	for _, entry := range entries {
		if !entry.Type().IsRegular() || !strings.HasPrefix(entry.Name(), tempPrefix) {
			continue
		}
		info, err := entry.Info()
		if err != nil || time.Since(info.ModTime()) < time.Minute {
			continue
		}
		if err := os.Remove(filepath.Join(dir, entry.Name())); err != nil && !errors.Is(err, fs.ErrNotExist) {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// writeFile writes content to a temporary file and renames it to file, so
// readers never see a partial file.
func writeFile(file string, content []byte) (err error) {
	name := filepath.Join(filepath.Dir(file), tempPrefix+rand.Text())
	tmp, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644) //nolint:gosec // the file name is a random suffix in the store directory
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = os.Remove(name)
		}
	}()
	if _, err = tmp.Write(content); err == nil {
		err = tmp.Sync()
	}
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	return os.Rename(name, file)
}

// Read returns the content of the file named hash. It returns ErrInvalidHash
// for an invalid hash and an error matching fs.ErrNotExist for a missing file.
func (s *Store) Read(ctx context.Context, hash string) ([]byte, error) {
	if !hashPattern.MatchString(hash) {
		return nil, ErrInvalidHash
	}
	var content []byte
	err := s.do(ctx, "store read", hash, func(file string) (err error) {
		content, err = os.ReadFile(file) //nolint:gosec // the file name is a validated hash in the store directory
		return err
	})
	if err != nil {
		return nil, err
	}
	return content, nil
}

func (s *Store) do(ctx context.Context, name, hash string, op func(file string) error) error {
	file := filepath.Join(s.dir, hash)
	_, span := s.tracer.Start(ctx, name, trace.WithAttributes(semconv.FilePath(file)))
	defer span.End()
	err := op(file)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return err
}
