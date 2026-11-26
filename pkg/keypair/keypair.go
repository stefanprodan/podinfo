package keypair

import (
	"crypto/tls"
	"os"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Reloader serves a TLS key pair from disk and reloads it when the files change.
type Reloader struct {
	certFile string
	keyFile  string
	logger   *zap.Logger
	interval time.Duration

	mu       sync.Mutex
	cert     *tls.Certificate
	certInfo os.FileInfo
	keyInfo  os.FileInfo
	checked  time.Time
	failing  bool
}

// NewReloader loads the key pair from certFile and keyFile.
func NewReloader(certFile, keyFile string, logger *zap.Logger) (*Reloader, error) {
	r := &Reloader{certFile: certFile, keyFile: keyFile, logger: logger, interval: time.Second}
	if err := r.reload(); err != nil {
		return nil, err
	}
	return r, nil
}

// GetCertificate returns the current key pair, reloading it at most once per
// interval if the files changed. A pair that fails to load keeps the previous one.
func (r *Reloader) GetCertificate(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if time.Since(r.checked) < r.interval {
		return r.cert, nil
	}
	r.checked = time.Now()

	err := r.reload()
	switch {
	case err != nil && !r.failing:
		r.failing = true
		r.logger.Warn("reloading the TLS key pair failed, serving the previous one",
			zap.String("cert", r.certFile), zap.Error(err))
	case err == nil && r.failing:
		r.failing = false
		r.logger.Info("TLS key pair reload recovered", zap.String("cert", r.certFile))
	}
	return r.cert, nil
}

// TLSConfig returns a server TLS config that serves the current key pair.
func (r *Reloader) TLSConfig() *tls.Config {
	return &tls.Config{GetCertificate: r.GetCertificate}
}

func (r *Reloader) reload() error {
	certInfo, err := os.Stat(r.certFile)
	if err != nil {
		return err
	}
	keyInfo, err := os.Stat(r.keyFile)
	if err != nil {
		return err
	}
	if r.cert != nil && unchanged(r.certInfo, certInfo) && unchanged(r.keyInfo, keyInfo) {
		return nil
	}

	cert, err := tls.LoadX509KeyPair(r.certFile, r.keyFile)
	if err != nil {
		return err
	}
	r.cert = &cert
	r.certInfo = certInfo
	r.keyInfo = keyInfo
	return nil
}

func unchanged(old, cur os.FileInfo) bool {
	return os.SameFile(old, cur) && old.ModTime().Equal(cur.ModTime()) && old.Size() == cur.Size()
}
