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
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/stefanprodan/podinfo/pkg/fscache"
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

func writeFile(t *testing.T, name string, data []byte) {
	t.Helper()
	if err := os.WriteFile(name, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func writePair(t *testing.T, dir string, p pair) {
	t.Helper()
	writeFile(t, filepath.Join(dir, "tls.crt"), p.cert)
	writeFile(t, filepath.Join(dir, "tls.key"), p.key)
}

func newKeyPair(t *testing.T, dir string) (*KeyPair, *observer.ObservedLogs) {
	t.Helper()
	core, logs := observer.New(zap.InfoLevel)
	kp, err := New(dir, zap.New(core), fscache.WithDebounce(20*time.Millisecond), fscache.WithResync(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = kp.Close() })
	return kp, logs
}

func commonName(t *testing.T, kp *KeyPair) string {
	t.Helper()
	cert, err := kp.GetCertificate(nil)
	if err != nil {
		t.Fatal(err)
	}
	leaf, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	return leaf.Subject.CommonName
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

func TestKeyPairRotation(t *testing.T) {
	dir := t.TempDir()
	writePair(t, dir, newPair(t, "first", nil))
	kp, _ := newKeyPair(t, dir)

	if cn := commonName(t, kp); cn != "first" {
		t.Fatalf("got %q, want first", cn)
	}

	writePair(t, dir, newPair(t, "second", nil))
	eventually(t, "the rewritten pair was not loaded", func() bool { return commonName(t, kp) == "second" })
}

func TestKeyPairSymlinkSwap(t *testing.T) {
	dir := t.TempDir()
	for v, cn := range map[string]string{"..v1": "v1", "..v2": "v2"} {
		if err := os.Mkdir(filepath.Join(dir, v), 0o700); err != nil {
			t.Fatal(err)
		}
		writePair(t, filepath.Join(dir, v), newPair(t, cn, nil))
	}
	data := filepath.Join(dir, "..data")
	if err := os.Symlink("..v1", data); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"tls.crt", "tls.key"} {
		if err := os.Symlink(filepath.Join("..data", f), filepath.Join(dir, f)); err != nil {
			t.Fatal(err)
		}
	}
	kp, _ := newKeyPair(t, dir)

	if cn := commonName(t, kp); cn != "v1" {
		t.Fatalf("got %q, want v1", cn)
	}

	tmp := filepath.Join(dir, "..data_tmp")
	if err := os.Symlink("..v2", tmp); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(tmp, data); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(dir, "..v1")); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the ..data swap was not picked up", func() bool { return commonName(t, kp) == "v2" })
}

func TestKeyPairMismatched(t *testing.T) {
	dir := t.TempDir()
	writePair(t, dir, newPair(t, "first", nil))
	kp, logs := newKeyPair(t, dir)

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	next := newPair(t, "second", key)

	writeFile(t, filepath.Join(dir, "tls.key"), next.key)
	eventually(t, "the mismatched pair was not reported", func() bool { return logs.FilterMessageSnippet("failed").Len() == 1 })
	if cn := commonName(t, kp); cn != "first" {
		t.Fatalf("got %q with a mismatched key, want first", cn)
	}

	writeFile(t, filepath.Join(dir, "tls.crt"), next.cert)
	eventually(t, "the matching pair was not loaded", func() bool { return commonName(t, kp) == "second" })
	if n := logs.FilterMessage("reloaded the TLS key pair").Len(); n != 1 {
		t.Fatalf("got %d recovery logs, want 1", n)
	}
	if n := logs.FilterMessageSnippet("failed").Len(); n != 1 {
		t.Fatalf("got %d failure logs, want 1", n)
	}
}

func TestKeyPairConcurrent(t *testing.T) {
	dir := t.TempDir()
	writePair(t, dir, newPair(t, "first", nil))
	kp, _ := newKeyPair(t, dir)

	stop := make(chan struct{})
	var wg sync.WaitGroup
	defer func() { close(stop); wg.Wait() }()
	for range 8 {
		wg.Go(func() {
			for {
				select {
				case <-stop:
					return
				default:
				}
				if cert, err := kp.GetCertificate(nil); err != nil || cert == nil {
					t.Error("GetCertificate returned no certificate")
					return
				}
			}
		})
	}
	writePair(t, dir, newPair(t, "second", nil))
	eventually(t, "the rewritten pair was not loaded", func() bool { return commonName(t, kp) == "second" })
}

func TestKeyPairIgnoresOtherFiles(t *testing.T) {
	dir := t.TempDir()
	writePair(t, dir, newPair(t, "first", nil))
	if err := os.Symlink("missing", filepath.Join(dir, "ca.crt")); err != nil {
		t.Fatal(err)
	}
	kp, logs := newKeyPair(t, dir)

	writePair(t, dir, newPair(t, "second", nil))
	eventually(t, "the rewritten pair was not loaded", func() bool { return commonName(t, kp) == "second" })
	if n := logs.FilterMessageSnippet("directory").Len(); n != 0 {
		t.Fatalf("got %d directory warnings for an unrelated file, want 0", n)
	}
}

func TestKeyPairInvalid(t *testing.T) {
	dir := t.TempDir()
	if _, err := New(dir, zap.NewNop()); err == nil {
		t.Fatal("expected an error without tls.crt and tls.key")
	}

	writeFile(t, filepath.Join(dir, "tls.crt"), []byte("invalid"))
	writeFile(t, filepath.Join(dir, "tls.key"), []byte("invalid"))
	if _, err := New(dir, zap.NewNop()); err == nil {
		t.Fatal("expected an error for an invalid pair")
	}
}
