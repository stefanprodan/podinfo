// Copyright 2018 Stefan Prodan.
// SPDX-License-Identifier: Apache-2.0

package fscache

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"
	"go.uber.org/zap"
)

const dataLink = "..data"

// Watcher keeps an in-memory snapshot of the files in a directory, refreshed on change.
type Watcher struct {
	dir      string
	logger   *zap.Logger
	names    []string
	debounce time.Duration
	resync   time.Duration
	resyncC  <-chan time.Time

	fsw     *fsnotify.Watcher
	watched bool
	files   atomic.Pointer[map[string][]byte]
	failing bool

	mu          sync.Mutex
	subscribers []func(map[string][]byte)
	notify      chan struct{}

	started   atomic.Bool
	stop      chan struct{}
	done      chan struct{}
	closeOnce sync.Once
	closeErr  error
}

// Option configures a Watcher.
type Option func(*Watcher)

// WithDebounce sets how long the Watcher waits for events to settle before rereading the directory.
func WithDebounce(d time.Duration) Option {
	return func(w *Watcher) { w.debounce = d }
}

// WithResync sets how often the Watcher rereads the directory regardless of events.
func WithResync(d time.Duration) Option {
	return func(w *Watcher) { w.resync = d }
}

// WithFiles limits the Watcher to the named files, which must all exist.
func WithFiles(names ...string) Option {
	return func(w *Watcher) { w.names = slices.Clone(names) }
}

// New watches dir and reads its files; call Start to begin processing changes.
func New(dir string, logger *zap.Logger, opts ...Option) (*Watcher, error) {
	w := &Watcher{
		dir:      filepath.Clean(dir),
		logger:   logger,
		debounce: 100 * time.Millisecond,
		resync:   30 * time.Second,
		notify:   make(chan struct{}, 1),
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
	}
	for _, opt := range opts {
		opt(w)
	}
	if w.debounce <= 0 || w.resync <= 0 {
		return nil, fmt.Errorf("debounce and resync must be positive, got %s and %s", w.debounce, w.resync)
	}

	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	w.fsw = fsw
	if err := w.watch(); err != nil {
		_ = fsw.Close()
		return nil, err
	}

	files, err := w.read()
	if err != nil {
		_ = fsw.Close()
		return nil, err
	}
	w.files.Store(&files)
	return w, nil
}

// Files returns a copy of the current snapshot.
func (w *Watcher) Files() map[string][]byte {
	return clone(*w.files.Load())
}

// Subscribe registers fn for the latest snapshot: calls may be coalesced, run serially, must not block, and one may still run after Close.
func (w *Watcher) Subscribe(fn func(map[string][]byte)) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.subscribers = append(w.subscribers, fn)
}

// Start processes directory changes in the background until Close is called.
func (w *Watcher) Start() {
	if w.started.Swap(true) {
		return
	}
	go w.run()
	go w.dispatch()
}

// Close stops watching the directory; it doesn't wait for a running callback.
func (w *Watcher) Close() error {
	w.closeOnce.Do(func() {
		close(w.stop)
		if w.started.Load() {
			<-w.done
		}
		w.closeErr = w.fsw.Close()
	})
	return w.closeErr
}

func (w *Watcher) run() {
	defer close(w.done)

	resyncC := w.resyncC
	if resyncC == nil {
		resync := time.NewTicker(w.resync)
		defer resync.Stop()
		resyncC = resync.C
	}
	debounce := time.NewTimer(w.debounce)
	debounce.Stop()
	defer debounce.Stop()
	var pending time.Time

	for {
		select {
		case <-w.stop:
			return
		case event, ok := <-w.fsw.Events:
			if !ok {
				return
			}
			if event.Op == fsnotify.Chmod {
				continue
			}
			if event.Name == w.dir && event.Op.Has(fsnotify.Remove|fsnotify.Rename) {
				w.watched = false
			}
			if pending.IsZero() {
				pending = time.Now()
			}
			debounce.Reset(max(0, min(w.debounce, 10*w.debounce-time.Since(pending))))
		case err, ok := <-w.fsw.Errors:
			if !ok {
				return
			}
			w.logger.Warn("watching the directory failed", zap.String("dir", w.dir), zap.Error(err))
			if errors.Is(err, fsnotify.ErrEventOverflow) {
				w.watched = false
				pending = time.Time{}
				debounce.Stop()
				w.reload()
			}
		case <-debounce.C:
			pending = time.Time{}
			w.reload()
		case <-resyncC:
			w.reload()
		}
	}
}

func (w *Watcher) dispatch() {
	for {
		select {
		case <-w.stop:
			return
		case <-w.notify:
			w.mu.Lock()
			subscribers := w.subscribers
			w.mu.Unlock()
			for _, fn := range subscribers {
				select {
				case <-w.stop:
					return
				default:
				}
				fn(w.Files())
			}
		}
	}
}

func (w *Watcher) reload() {
	if err := w.watch(); err != nil {
		w.fail(err)
		return
	}

	files, err := w.read()
	if err != nil {
		w.fail(err)
		return
	}
	if w.failing {
		w.failing = false
		w.logger.Info("reading the directory recovered", zap.String("dir", w.dir))
	}

	if maps.EqualFunc(files, *w.files.Load(), bytes.Equal) {
		return
	}
	w.files.Store(&files)
	select {
	case w.notify <- struct{}{}:
	default:
	}
}

func (w *Watcher) fail(err error) {
	if !w.failing {
		w.failing = true
		w.logger.Warn("reading the directory failed, keeping the previous files", zap.String("dir", w.dir), zap.Error(err))
	}
}

func (w *Watcher) watch() error {
	if w.watched {
		return nil
	}
	_ = w.fsw.Remove(w.dir)
	if err := w.fsw.Add(w.dir); err != nil {
		return err
	}
	w.watched = true
	return nil
}

func (w *Watcher) read() (map[string][]byte, error) {
	for range 3 {
		base, err := generation(w.dir)
		if err != nil {
			return nil, err
		}
		files, err := readFiles(base, w.names)
		if now, nowErr := generation(w.dir); nowErr == nil && now == base {
			return files, err
		}
	}
	return nil, fmt.Errorf("%s kept changing while reading %s", dataLink, w.dir)
}

func generation(dir string) (string, error) {
	link := filepath.Join(dir, dataLink)
	info, err := os.Lstat(link)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return dir, nil
	case err != nil:
		return "", err
	case info.Mode()&fs.ModeSymlink == 0:
		return dir, nil
	}
	return filepath.EvalSymlinks(link)
}

func readFiles(dir string, names []string) (map[string][]byte, error) {
	if names == nil {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			if name := entry.Name(); !strings.HasPrefix(name, "..") {
				names = append(names, name)
			}
		}
	}

	files := make(map[string][]byte, len(names))
	for _, name := range names {
		path := filepath.Join(dir, name)
		info, err := os.Stat(path)
		if err != nil {
			return nil, err
		}
		if !info.Mode().IsRegular() {
			continue
		}
		data, err := os.ReadFile(path) //nolint:gosec // the watched directory and file names come from the config
		if err != nil {
			return nil, err
		}
		files[name] = data
	}
	return files, nil
}

func clone(files map[string][]byte) map[string][]byte {
	out := make(map[string][]byte, len(files))
	for name, data := range files {
		out[name] = bytes.Clone(data)
	}
	return out
}
