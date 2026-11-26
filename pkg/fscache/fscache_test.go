package fscache

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

const (
	testDebounce = 20 * time.Millisecond
	testResync   = time.Hour
)

type recorder struct {
	mu    sync.Mutex
	calls []map[string][]byte
}

func (r *recorder) record(files map[string][]byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, files)
}

func (r *recorder) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.calls)
}

func newWatcher(t *testing.T, dir string, opts ...Option) (*Watcher, *recorder, *observer.ObservedLogs) {
	t.Helper()
	core, logs := observer.New(zap.InfoLevel)
	opts = append([]Option{WithDebounce(testDebounce), WithResync(testResync)}, opts...)
	w, err := New(dir, zap.New(core), opts...)
	if err != nil {
		t.Fatal(err)
	}
	rec := &recorder{}
	w.Subscribe(rec.record)
	w.Start()
	t.Cleanup(func() { _ = w.Close() })
	return w, rec, logs
}

func writeFile(t *testing.T, name, data string) {
	t.Helper()
	if err := os.WriteFile(name, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
}

func mkdir(t *testing.T, name string) {
	t.Helper()
	if err := os.MkdirAll(name, 0o700); err != nil {
		t.Fatal(err)
	}
}

func symlink(t *testing.T, target, name string) {
	t.Helper()
	if err := os.Symlink(target, name); err != nil {
		t.Fatal(err)
	}
}

func swapLink(t *testing.T, target, name string) {
	t.Helper()
	tmp := name + "_tmp"
	if err := os.Symlink(target, tmp); err != nil {
		t.Error(err)
		return
	}
	if err := os.Rename(tmp, name); err != nil {
		t.Error(err)
	}
}

func eventually(t *testing.T, msg string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal(msg)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func content(w *Watcher, name string) string {
	return string(w.Files()[name])
}

func TestWatcherPlainDir(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a"), "1")
	mkdir(t, filepath.Join(dir, "sub"))
	w, rec, _ := newWatcher(t, dir)

	if got := w.Files(); len(got) != 1 || content(w, "a") != "1" {
		t.Fatalf("got %v after the initial read, want only a=1", got)
	}

	writeFile(t, filepath.Join(dir, "b"), "2")
	eventually(t, "file b was not added", func() bool { return content(w, "b") == "2" })

	writeFile(t, filepath.Join(dir, "a"), "3")
	eventually(t, "file a was not updated", func() bool { return content(w, "a") == "3" })

	if err := os.Remove(filepath.Join(dir, "b")); err != nil {
		t.Fatal(err)
	}
	eventually(t, "file b was not removed", func() bool { _, ok := w.Files()["b"]; return !ok })
	if got := w.Files(); len(got) != 1 || content(w, "a") != "3" {
		t.Fatalf("got %v, want only a=3", got)
	}
	eventually(t, "subscribers were not notified", func() bool { return rec.count() >= 1 })
}

func TestWatcherFilesCopy(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a"), "1")
	w, _, _ := newWatcher(t, dir)

	files := w.Files()
	files["a"][0] = 'x'
	files["b"] = []byte("2")
	if got := w.Files(); len(got) != 1 || content(w, "a") != "1" {
		t.Fatalf("got %v after modifying a copy, want only a=1", got)
	}
}

func TestWatcherSymlinkSwap(t *testing.T) {
	dir := t.TempDir()
	for _, v := range []string{"..v1", "..v2"} {
		mkdir(t, filepath.Join(dir, v))
		writeFile(t, filepath.Join(dir, v, "config"), v)
	}
	symlink(t, "..v1", filepath.Join(dir, dataLink))
	symlink(t, filepath.Join(dataLink, "config"), filepath.Join(dir, "config"))
	w, _, _ := newWatcher(t, dir)

	if got := w.Files(); len(got) != 1 || content(w, "config") != "..v1" {
		t.Fatalf("got %v, want only config=..v1", got)
	}

	swapLink(t, "..v2", filepath.Join(dir, dataLink))
	if err := os.RemoveAll(filepath.Join(dir, "..v1")); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the ..data swap was not picked up", func() bool { return content(w, "config") == "..v2" })
}

func TestWatcherConsistentGenerations(t *testing.T) {
	dir := t.TempDir()
	writeGeneration := func(i int) string {
		gen := fmt.Sprintf("..gen%d", i)
		mkdir(t, filepath.Join(dir, gen))
		writeFile(t, filepath.Join(dir, gen, "a"), fmt.Sprint(i))
		writeFile(t, filepath.Join(dir, gen, "b"), fmt.Sprint(i))
		return gen
	}
	symlink(t, writeGeneration(0), filepath.Join(dir, dataLink))
	symlink(t, filepath.Join(dataLink, "a"), filepath.Join(dir, "a"))
	symlink(t, filepath.Join(dataLink, "b"), filepath.Join(dir, "b"))

	var mixed atomic.Int32
	check := func(files map[string][]byte) {
		if string(files["a"]) != string(files["b"]) {
			mixed.Add(1)
		}
	}
	w, rec, _ := newWatcher(t, dir, WithDebounce(time.Millisecond))
	w.Subscribe(check)

	stop := make(chan struct{})
	var wg sync.WaitGroup
	halt := sync.OnceFunc(func() { close(stop); wg.Wait() })
	defer halt()
	wg.Go(func() {
		for i := 1; ; i++ {
			select {
			case <-stop:
				return
			case <-time.After(2 * time.Millisecond):
			}
			swapLink(t, writeGeneration(i), filepath.Join(dir, dataLink))
			if err := os.RemoveAll(filepath.Join(dir, fmt.Sprintf("..gen%d", i-1))); err != nil {
				t.Error(err)
			}
		}
	})
	wg.Go(func() {
		for {
			select {
			case <-stop:
				return
			default:
			}
			check(w.Files())
		}
	})
	eventually(t, "no generation swaps were observed", func() bool { return rec.count() >= 10 })

	halt()
	if n := mixed.Load(); n > 0 {
		t.Fatalf("got %d snapshots mixing two generations", n)
	}
}

func TestWatcherReadsOneGeneration(t *testing.T) {
	dir := t.TempDir()
	for _, v := range []string{"..v1", "..v2"} {
		mkdir(t, filepath.Join(dir, v))
		writeFile(t, filepath.Join(dir, v, "a"), v)
		writeFile(t, filepath.Join(dir, v, "b"), v)
	}
	symlink(t, "..v2", filepath.Join(dir, dataLink))
	symlink(t, filepath.Join(dataLink, "a"), filepath.Join(dir, "a"))
	symlink(t, filepath.Join("..v1", "b"), filepath.Join(dir, "b"))
	w, _, _ := newWatcher(t, dir)

	if a, b := content(w, "a"), content(w, "b"); a != "..v2" || b != "..v2" {
		t.Fatalf("got a=%s b=%s, want both from ..v2", a, b)
	}
}

func TestWatcherWithFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a"), "1")
	writeFile(t, filepath.Join(dir, "other"), "x")
	symlink(t, "missing", filepath.Join(dir, "dangling"))
	w, _, logs := newWatcher(t, dir, WithFiles("a"), WithResync(10*time.Millisecond))

	if got := w.Files(); len(got) != 1 || content(w, "a") != "1" {
		t.Fatalf("got %v, want only a=1", got)
	}

	writeFile(t, filepath.Join(dir, "a"), "2")
	eventually(t, "file a was not updated", func() bool { return content(w, "a") == "2" })
	if n := logs.FilterMessageSnippet("failed").Len(); n != 0 {
		t.Fatalf("got %d failure logs for unrelated files, want 0", n)
	}

	if _, err := New(dir, zap.NewNop(), WithFiles("missing")); err == nil {
		t.Fatal("expected an error for a missing named file")
	}
}

func TestWatcherRootRecreate(t *testing.T) {
	for name, suffix := range map[string]string{"clean": "", "trailing slash": "/"} {
		t.Run(name, func(t *testing.T) { testRootRecreate(t, suffix) })
	}
}

func testRootRecreate(t *testing.T, suffix string) {
	dir := filepath.Join(t.TempDir(), "root")
	mkdir(t, dir)
	writeFile(t, filepath.Join(dir, "a"), "1")
	resync := 2 * time.Second
	w, _, logs := newWatcher(t, dir+suffix, WithResync(resync))

	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the removed root was not reported", func() bool { return logs.FilterMessageSnippet("reading the directory failed").Len() == 1 })
	if content(w, "a") != "1" {
		t.Fatalf("got %v after the root was removed, want the previous files", w.Files())
	}

	mkdir(t, dir)
	writeFile(t, filepath.Join(dir, "a"), "2")
	eventually(t, "the recreated root was not read", func() bool { return content(w, "a") == "2" })

	start := time.Now()
	writeFile(t, filepath.Join(dir, "a"), "3")
	eventually(t, "the recreated root is not watched", func() bool { return content(w, "a") == "3" })
	if elapsed := time.Since(start); elapsed > resync/4 {
		t.Fatalf("the edit took %s, want it event-driven, well before the %s resync", elapsed, resync)
	}
}

func TestWatcherMaxWait(t *testing.T) {
	dir := t.TempDir()
	w, _, _ := newWatcher(t, dir, WithDebounce(50*time.Millisecond))

	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Go(func() {
		for i := 0; ; i++ {
			select {
			case <-stop:
				return
			case <-time.After(5 * time.Millisecond):
				writeFile(t, filepath.Join(dir, "a"), fmt.Sprint(i))
			}
		}
	})
	defer func() { close(stop); wg.Wait() }()
	eventually(t, "continuous writes postponed the reread", func() bool { return content(w, "a") != "" })
}

func TestWatcherReadError(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a"), "1")
	w, _, logs := newWatcher(t, dir, WithResync(10*time.Millisecond))

	symlink(t, "missing", filepath.Join(dir, "broken"))
	eventually(t, "the read error was not logged", func() bool { return logs.FilterMessageSnippet("failed").Len() > 0 })
	time.Sleep(100 * time.Millisecond)
	if n := logs.FilterMessageSnippet("failed").Len(); n != 1 {
		t.Fatalf("got %d failure logs, want 1", n)
	}
	if content(w, "a") != "1" {
		t.Fatalf("got %v after a read error, want the previous files", w.Files())
	}

	if err := os.Remove(filepath.Join(dir, "broken")); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the recovery was not logged", func() bool { return logs.FilterMessageSnippet("recovered").Len() == 1 })
}

func TestWatcherClose(t *testing.T) {
	dir := t.TempDir()
	w, rec, _ := newWatcher(t, dir)

	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-w.done:
	default:
		t.Fatal("the watcher goroutine is still running after Close")
	}
	if err := w.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}

	writeFile(t, filepath.Join(dir, "a"), "1")
	time.Sleep(50 * time.Millisecond)
	if rec.count() != 0 {
		t.Fatalf("got %d notifications after Close, want 0", rec.count())
	}
}

func TestWatcherCloseFromCallback(t *testing.T) {
	dir := t.TempDir()
	w, err := New(dir, zap.NewNop(), WithDebounce(testDebounce))
	if err != nil {
		t.Fatal(err)
	}
	closed := make(chan error, 1)
	w.Subscribe(func(map[string][]byte) { closed <- w.Close() })
	w.Start()

	writeFile(t, filepath.Join(dir, "a"), "1")
	select {
	case err := <-closed:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Close from a callback deadlocked")
	}
}

func TestWatcherCloseBlockedCallback(t *testing.T) {
	dir := t.TempDir()
	w, err := New(dir, zap.NewNop(), WithDebounce(testDebounce))
	if err != nil {
		t.Fatal(err)
	}
	called, release := make(chan struct{}), make(chan struct{})
	defer close(release)
	w.Subscribe(func(map[string][]byte) {
		close(called)
		<-release
	})
	w.Start()

	writeFile(t, filepath.Join(dir, "a"), "1")
	select {
	case <-called:
	case <-time.After(5 * time.Second):
		t.Fatal("the callback was not called")
	}
	closed := make(chan error, 1)
	go func() { closed <- w.Close() }()
	select {
	case <-closed:
	case <-time.After(5 * time.Second):
		t.Fatal("a blocked callback blocked Close")
	}
}

func TestWatcherCloseWithoutStart(t *testing.T) {
	w, err := New(t.TempDir(), zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestWatcherInvalidOptions(t *testing.T) {
	for _, opt := range []Option{WithDebounce(0), WithResync(-time.Second)} {
		if _, err := New(t.TempDir(), zap.NewNop(), opt); err == nil {
			t.Fatal("expected an error for a non-positive duration")
		}
	}
}

func TestWatcherMissingDir(t *testing.T) {
	if _, err := New(filepath.Join(t.TempDir(), "missing"), zap.NewNop()); err == nil {
		t.Fatal("expected an error for a missing directory")
	}
}
