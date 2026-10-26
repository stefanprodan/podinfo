package http

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"go.uber.org/zap"
)

// artifact is an in-memory tar.gz of a directory and its digest.
type artifact struct {
	data   []byte
	digest string
}

// buildArtifact packs the regular files of dir into a deterministic tar.gz.
// Paths are relative to dir and sorted, and all metadata that could vary
// between builds is fixed, so the same files always give the same digest.
// Symlinks, directories and other file types are skipped.
func buildArtifact(dir string) (*artifact, error) {
	var files []string
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type().IsRegular() {
			files = append(files, p)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	names := make(map[string]string, len(files))
	for _, p := range files {
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return nil, err
		}
		names[p] = filepath.ToSlash(rel)
	}
	sort.Slice(files, func(i, j int) bool { return names[files[i]] < names[files[j]] })

	var buf bytes.Buffer
	gz, err := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	if err != nil {
		return nil, err
	}
	tw := tar.NewWriter(gz)
	for _, p := range files {
		content, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		hdr := &tar.Header{
			Typeflag: tar.TypeReg,
			Name:     names[p],
			Mode:     0o644,
			Size:     int64(len(content)),
			ModTime:  time.Unix(0, 0),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return nil, fmt.Errorf("writing tar header for %s: %w", names[p], err)
		}
		if _, err := tw.Write(content); err != nil {
			return nil, fmt.Errorf("writing tar content for %s: %w", names[p], err)
		}
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}

	sum := sha256.Sum256(buf.Bytes())
	return &artifact{
		data:   buf.Bytes(),
		digest: "sha256:" + hex.EncodeToString(sum[:]),
	}, nil
}

// loadArtifact builds the artifact from --artifact-path.
// Failures are logged and leave the route returning 404.
func (s *Server) loadArtifact() {
	if s.config.ArtifactPath == "" {
		return
	}
	a, err := buildArtifact(s.config.ArtifactPath)
	if err != nil {
		s.logger.Warn("artifact build failed, /artifact will return 404",
			zap.Error(err), zap.String("path", s.config.ArtifactPath))
		return
	}
	s.artifact = a
	s.logger.Info("Artifact loaded",
		zap.String("path", s.config.ArtifactPath),
		zap.String("digest", a.digest),
		zap.Int("size", len(a.data)))
}

// Artifact godoc
// @Summary Download artifact
// @Description returns the files from the artifact path as a tar.gz, the digest is set in the X-Artifact-Digest header
// @Tags HTTP API
// @Produce application/gzip
// @Router /artifact [get]
// @Success 200 {file} file "tar.gz archive"
// @Failure 404 {object} http.MapResponse
func (s *Server) artifactHandler(w http.ResponseWriter, r *http.Request) {
	_, span := s.tracer.Start(r.Context(), "artifactHandler")
	defer span.End()

	a := s.artifact
	if a == nil {
		s.JSONResponseCode(w, r, map[string]any{
			"code":    http.StatusNotFound,
			"message": "artifact not found",
		}, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("Content-Length", strconv.Itoa(len(a.data)))
	w.Header().Set("X-Artifact-Digest", a.digest)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		w.Write(a.data)
	}
}
