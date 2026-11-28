package spiffetest

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spiffe/go-spiffe/v2/bundle/x509bundle"
	"github.com/spiffe/go-spiffe/v2/proto/spiffe/workload"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/svid/x509svid"
	"github.com/spiffe/go-spiffe/v2/workloadapi"
	"google.golang.org/grpc"
)

// CA issues X.509 SVIDs for a single trust domain.
type CA struct {
	cert *x509.Certificate
	key  *ecdsa.PrivateKey
}

// NewCA returns a self-signed CA.
func NewCA(t *testing.T) *CA {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	serial, _ := rand.Int(rand.Reader, big.NewInt(1<<62))
	tmpl := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: "test-ca-" + serial.String()},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	return &CA{cert: cert, key: key}
}

// NewSVID issues an X.509 SVID for id.
func (ca *CA) NewSVID(t *testing.T, id spiffeid.ID) *x509svid.SVID {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	uri, err := url.Parse(id.String())
	if err != nil {
		t.Fatal(err)
	}
	serial, _ := rand.Int(rand.Reader, big.NewInt(1<<62))
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		URIs:         []*url.URL{uri},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, ca.cert, &key.PublicKey, ca.key)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	return &x509svid.SVID{ID: id, Certificates: []*x509.Certificate{cert}, PrivateKey: key}
}

// Bundle returns the CA as the X.509 bundle of td.
func (ca *CA) Bundle(td spiffeid.TrustDomain) *x509bundle.Bundle {
	return x509bundle.FromX509Authorities(td, []*x509.Certificate{ca.cert})
}

type fakeWorkloadAPI struct {
	workload.UnimplementedSpiffeWorkloadAPIServer
	resp *workload.X509SVIDResponse
}

func (f *fakeWorkloadAPI) FetchX509SVID(_ *workload.X509SVIDRequest,
	stream grpc.ServerStreamingServer[workload.X509SVIDResponse]) error {
	if err := stream.Send(f.resp); err != nil {
		return err
	}
	<-stream.Context().Done()
	return nil
}

// SocketPath returns a unix socket path in a temporary directory.
func SocketPath(t *testing.T) string {
	t.Helper()
	// unix socket paths are limited to ~104 bytes, so avoid long TMPDIRs
	base := ""
	if len(os.TempDir()) > 60 {
		base = "/tmp"
	}
	dir, err := os.MkdirTemp(base, "podinfo-spiffe") //nolint:usetesting // t.TempDir paths can exceed the unix socket path limit
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return filepath.Join(dir, "api.sock")
}

// StartWorkloadAPI serves svid (signed by ca) on socketPath.
func StartWorkloadAPI(t *testing.T, socketPath string, ca *CA, svid *x509svid.SVID) {
	t.Helper()
	keyDER, err := x509.MarshalPKCS8PrivateKey(svid.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	handler := &fakeWorkloadAPI{resp: &workload.X509SVIDResponse{Svids: []*workload.X509SVID{{
		SpiffeId:    svid.ID.String(),
		X509Svid:    svid.Certificates[0].Raw,
		X509SvidKey: keyDER,
		Bundle:      ca.cert.Raw,
	}}}}

	ln, err := (&net.ListenConfig{}).Listen(t.Context(), "unix", socketPath)
	if err != nil {
		t.Fatal(err)
	}
	server := grpc.NewServer()
	workload.RegisterSpiffeWorkloadAPIServer(server, handler)
	go func() { _ = server.Serve(ln) }()
	t.Cleanup(server.Stop)
}

// FastBackoff retries the Workload API every 10ms.
func FastBackoff() workloadapi.ClientOption {
	return workloadapi.WithBackoffStrategy(fastBackoff{})
}

type fastBackoff struct{}

func (fastBackoff) NewBackoff() workloadapi.Backoff { return fastBackoff{} }
func (fastBackoff) Next() time.Duration             { return 10 * time.Millisecond }
func (fastBackoff) Reset()                          {}
