package keypair

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

type pair struct {
	cert []byte
	key  []byte
}

func newPair(t *testing.T, cn string, key *ecdsa.PrivateKey) pair {
	t.Helper()
	if key == nil {
		var err error
		if key, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader); err != nil {
			t.Fatal(err)
		}
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: cn},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	return pair{
		cert: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		key:  pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}),
	}
}

func writeFile(t *testing.T, name string, data []byte, mod time.Time) {
	t.Helper()
	if err := os.WriteFile(name, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(name, mod, mod); err != nil {
		t.Fatal(err)
	}
}

func writePair(t *testing.T, dir string, p pair, mod time.Time) {
	t.Helper()
	writeFile(t, filepath.Join(dir, "tls.crt"), p.cert, mod)
	writeFile(t, filepath.Join(dir, "tls.key"), p.key, mod)
}

func newReloader(t *testing.T, dir string) (*Reloader, *observer.ObservedLogs) {
	t.Helper()
	core, logs := observer.New(zap.InfoLevel)
	r, err := NewReloader(filepath.Join(dir, "tls.crt"), filepath.Join(dir, "tls.key"), zap.New(core))
	if err != nil {
		t.Fatal(err)
	}
	r.interval = 0
	return r, logs
}

func commonName(t *testing.T, r *Reloader) string {
	t.Helper()
	cert, err := r.GetCertificate(nil)
	if err != nil {
		t.Fatal(err)
	}
	leaf, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	return leaf.Subject.CommonName
}

func TestReloader(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	writePair(t, dir, newPair(t, "first", nil), now)
	r, logs := newReloader(t, dir)

	if cn := commonName(t, r); cn != "first" {
		t.Fatalf("got %q, want first", cn)
	}

	writePair(t, dir, newPair(t, "second", nil), now.Add(time.Minute))
	if cn := commonName(t, r); cn != "second" {
		t.Fatalf("got %q after rotation, want second", cn)
	}

	writeFile(t, filepath.Join(dir, "tls.crt"), []byte("partial"), now.Add(2*time.Minute))
	if cn := commonName(t, r); cn != "second" {
		t.Fatalf("got %q after a bad write, want second", cn)
	}
	commonName(t, r)
	if n := logs.FilterMessageSnippet("failed").Len(); n != 1 {
		t.Fatalf("got %d failure logs, want 1", n)
	}

	writePair(t, dir, newPair(t, "third", nil), now.Add(3*time.Minute))
	if cn := commonName(t, r); cn != "third" {
		t.Fatalf("got %q after recovery, want third", cn)
	}
	if n := logs.FilterMessage("TLS key pair reload recovered").Len(); n != 1 {
		t.Fatalf("got %d recovery logs, want 1", n)
	}
}

func TestReloaderMismatchedPair(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	writePair(t, dir, newPair(t, "first", nil), now)
	r, logs := newReloader(t, dir)

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	next := newPair(t, "second", key)

	writeFile(t, filepath.Join(dir, "tls.key"), next.key, now.Add(time.Minute))
	if cn := commonName(t, r); cn != "first" {
		t.Fatalf("got %q with a mismatched key, want first", cn)
	}
	if n := logs.FilterMessageSnippet("failed").Len(); n != 1 {
		t.Fatalf("got %d failure logs for a key-only change, want 1", n)
	}

	writeFile(t, filepath.Join(dir, "tls.crt"), next.cert, now.Add(2*time.Minute))
	if cn := commonName(t, r); cn != "second" {
		t.Fatalf("got %q after the cert update, want second", cn)
	}
}

func TestReloaderSymlinkSwap(t *testing.T) {
	dir := t.TempDir()
	mod := time.Now()
	v1, v2 := newPair(t, "v1", nil), newPair(t, "v2", nil)
	v1.cert, v2.cert = padEqual(v1.cert, v2.cert)
	v1.key, v2.key = padEqual(v1.key, v2.key)
	for v, p := range map[string]pair{"v1": v1, "v2": v2} {
		if err := os.Mkdir(filepath.Join(dir, v), 0o700); err != nil {
			t.Fatal(err)
		}
		writePair(t, filepath.Join(dir, v), p, mod)
	}
	data := filepath.Join(dir, "..data")
	if err := os.Symlink("v1", data); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"tls.crt", "tls.key"} {
		if err := os.Symlink(filepath.Join("..data", f), filepath.Join(dir, f)); err != nil {
			t.Fatal(err)
		}
	}
	r, _ := newReloader(t, dir)

	if cn := commonName(t, r); cn != "v1" {
		t.Fatalf("got %q, want v1", cn)
	}

	tmp := filepath.Join(dir, "..data_tmp")
	if err := os.Symlink("v2", tmp); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(tmp, data); err != nil {
		t.Fatal(err)
	}
	if cn := commonName(t, r); cn != "v2" {
		t.Fatalf("got %q after the symlink swap, want v2", cn)
	}
}

func TestReloaderInterval(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	writePair(t, dir, newPair(t, "first", nil), now)
	r, _ := newReloader(t, dir)
	r.interval = time.Hour
	commonName(t, r)

	writePair(t, dir, newPair(t, "second", nil), now.Add(time.Minute))
	if cn := commonName(t, r); cn != "first" {
		t.Fatalf("got %q within the interval, want first", cn)
	}

	r.checked = time.Now().Add(-2 * time.Hour)
	if cn := commonName(t, r); cn != "second" {
		t.Fatalf("got %q after the interval, want second", cn)
	}
}

func TestReloaderMissingFiles(t *testing.T) {
	dir := t.TempDir()
	if _, err := NewReloader(filepath.Join(dir, "tls.crt"), filepath.Join(dir, "tls.key"), zap.NewNop()); err == nil {
		t.Fatal("expected an error for missing files")
	}
}

func padEqual(a, b []byte) ([]byte, []byte) {
	for len(a) < len(b) {
		a = append(a, '\n')
	}
	for len(b) < len(a) {
		b = append(b, '\n')
	}
	return a, b
}
