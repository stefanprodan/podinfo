package keypair

import (
	"crypto/tls"
	"sync/atomic"

	"go.uber.org/zap"

	"github.com/stefanprodan/podinfo/pkg/fscache"
)

// KeyPair serves the tls.crt and tls.key of a directory and reloads them when they change.
type KeyPair struct {
	dir     string
	logger  *zap.Logger
	watcher *fscache.Watcher
	cert    atomic.Pointer[tls.Certificate]
	failing bool
}

// New loads the key pair from dir and watches it for changes.
func New(dir string, logger *zap.Logger, opts ...fscache.Option) (*KeyPair, error) {
	watcher, err := fscache.New(dir, logger, append([]fscache.Option{fscache.WithFiles("tls.crt", "tls.key")}, opts...)...)
	if err != nil {
		return nil, err
	}

	cert, err := parse(watcher.Files())
	if err != nil {
		_ = watcher.Close()
		return nil, err
	}

	kp := &KeyPair{dir: dir, logger: logger, watcher: watcher}
	kp.cert.Store(cert)
	watcher.Subscribe(kp.update)
	watcher.Start()
	return kp, nil
}

// GetCertificate returns the current key pair.
func (kp *KeyPair) GetCertificate(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	return kp.cert.Load(), nil
}

// TLSConfig returns a server TLS config that serves the current key pair.
func (kp *KeyPair) TLSConfig() *tls.Config {
	return &tls.Config{GetCertificate: kp.GetCertificate}
}

// Close stops watching the directory.
func (kp *KeyPair) Close() error {
	return kp.watcher.Close()
}

func (kp *KeyPair) update(files map[string][]byte) {
	cert, err := parse(files)
	switch {
	case err != nil:
		if !kp.failing {
			kp.failing = true
			kp.logger.Warn("reloading the TLS key pair failed, serving the previous one", zap.String("dir", kp.dir), zap.Error(err))
		}
		return
	case kp.failing:
		kp.failing = false
		kp.logger.Info("reloaded the TLS key pair", zap.String("dir", kp.dir))
	}
	kp.cert.Store(cert)
}

func parse(files map[string][]byte) (*tls.Certificate, error) {
	cert, err := tls.X509KeyPair(files["tls.crt"], files["tls.key"])
	if err != nil {
		return nil, err
	}
	return &cert, nil
}
