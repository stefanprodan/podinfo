package grpc

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/spiffetls/tlsconfig"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"

	headerv1 "github.com/stefanprodan/podinfo/pkg/api/grpc/header/v1"
	podv1 "github.com/stefanprodan/podinfo/pkg/api/grpc/pod/v1"
	"github.com/stefanprodan/podinfo/pkg/keypair"
	"github.com/stefanprodan/podinfo/pkg/spiffe"
	"github.com/stefanprodan/podinfo/pkg/spiffe/spiffetest"
)

const waitTimeout = 30 * time.Second

var (
	serverID = spiffeid.RequireFromString("spiffe://example.org/ns/default/sa/podinfo")
	clientID = spiffeid.RequireFromString("spiffe://example.org/ns/default/sa/client")
	otherID  = spiffeid.RequireFromString("spiffe://example.org/ns/default/sa/other")
)

// listen returns a listener on a free loopback port.
func listen(t *testing.T) net.Listener {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	return ln
}

// writeCert writes a self-signed tls.crt and tls.key for 127.0.0.1 to dir.
func writeCert(t *testing.T, dir string) *x509.Certificate {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "podinfo"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	if err := os.WriteFile(filepath.Join(dir, "tls.crt"), certPEM, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "tls.key"), keyPEM, 0o600); err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	return cert
}

// startTLSServer starts the gRPC server on a free port and stops it at the end of the test.
func startTLSServer(t *testing.T, s *Server) (string, *health.Server) {
	t.Helper()
	ln := listen(t)
	srv, healthSrv := s.serve(ln)
	t.Cleanup(srv.Stop)
	return ln.Addr().String(), healthSrv
}

func checkHealth(address string, creds credentials.TransportCredentials) (grpc_health_v1.HealthCheckResponse_ServingStatus, error) {
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(creds))
	if err != nil {
		return 0, err
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), waitTimeout)
	defer cancel()
	res, err := grpc_health_v1.NewHealthClient(conn).Check(ctx, &grpc_health_v1.HealthCheckRequest{Service: "podinfo"})
	if err != nil {
		return 0, err
	}
	return res.GetStatus(), nil
}

func serviceStatus(t *testing.T, healthSrv *health.Server) grpc_health_v1.HealthCheckResponse_ServingStatus {
	t.Helper()
	res, err := healthSrv.Check(context.Background(), &grpc_health_v1.HealthCheckRequest{Service: "podinfo"})
	if err != nil {
		t.Fatal(err)
	}
	return res.GetStatus()
}

func TestGrpcTLS_CertPath(t *testing.T) {
	dir := t.TempDir()
	cert := writeCert(t, dir)

	kp, err := keypair.New(dir, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = kp.Close() })

	s := NewServer(&Config{ServiceName: "podinfo", TLS: true}, zap.NewNop())
	s.SetKeyPair(kp)
	address, _ := startTLSServer(t, s)

	roots := x509.NewCertPool()
	roots.AddCert(cert)
	got, err := checkHealth(address, credentials.NewTLS(&tls.Config{RootCAs: roots}))
	if err != nil {
		t.Fatalf("TLS client failed: %v", err)
	}
	if got != grpc_health_v1.HealthCheckResponse_SERVING {
		t.Fatalf("got %s, want SERVING", got)
	}

	if _, err := checkHealth(address, insecure.NewCredentials()); err == nil {
		t.Fatal("expected a plaintext client to be rejected")
	}
	if _, err := checkHealth(address, credentials.NewTLS(&tls.Config{})); err == nil {
		t.Fatal("expected a client that doesn't trust the certificate to be rejected")
	}
}

func TestGrpcTLS_NoKeyPair(t *testing.T) {
	s := NewServer(&Config{TLS: true}, zap.NewNop())
	if _, err := s.credentials(); err == nil {
		t.Fatal("expected an error without a key pair")
	}
}

func TestGrpcTLS_Disabled(t *testing.T) {
	ca := spiffetest.NewCA(t)
	socket := spiffetest.SocketPath(t)
	spiffetest.StartWorkloadAPI(t, socket, ca, ca.NewSVID(t, serverID))
	source := spiffe.NewSource("unix://"+socket, nil, zap.NewNop(), spiffetest.FastBackoff())
	t.Cleanup(func() { _ = source.Close() })

	s := NewServer(&Config{ServiceName: "podinfo"}, zap.NewNop())
	s.SetSPIFFESource(source)
	address, _ := startTLSServer(t, s)

	got, err := checkHealth(address, insecure.NewCredentials())
	if err != nil {
		t.Fatalf("plaintext client failed without --grpc-tls: %v", err)
	}
	if got != grpc_health_v1.HealthCheckResponse_SERVING {
		t.Fatalf("got %s, want SERVING", got)
	}
}

func TestGrpcSPIFFEMTLS(t *testing.T) {
	ca := spiffetest.NewCA(t)
	bundle := ca.Bundle(serverID.TrustDomain())
	socket := spiffetest.SocketPath(t)
	source := spiffe.NewSource("unix://"+socket, []spiffeid.ID{clientID}, zap.NewNop(), spiffetest.FastBackoff())
	t.Cleanup(func() { _ = source.Close() })

	s := NewServer(&Config{ServiceName: "podinfo", TLS: true}, zap.NewNop())
	s.SetSPIFFESource(source)
	address, healthSrv := startTLSServer(t, s)

	if got := serviceStatus(t, healthSrv); got != grpc_health_v1.HealthCheckResponse_NOT_SERVING {
		t.Fatalf("before the SVID: got %s, want NOT_SERVING", got)
	}
	if _, err := checkHealth(address, credentials.NewTLS(tlsconfig.MTLSClientConfig(ca.NewSVID(t, clientID), bundle, tlsconfig.AuthorizeID(serverID)))); err == nil {
		t.Fatal("expected the handshake to fail before the SVID")
	}

	spiffetest.StartWorkloadAPI(t, socket, ca, ca.NewSVID(t, serverID))
	deadline := time.Now().Add(waitTimeout)
	for serviceStatus(t, healthSrv) != grpc_health_v1.HealthCheckResponse_SERVING {
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for SERVING")
		}
		time.Sleep(20 * time.Millisecond)
	}

	t.Run("authorized client", func(t *testing.T) {
		creds := credentials.NewTLS(tlsconfig.MTLSClientConfig(ca.NewSVID(t, clientID), bundle, tlsconfig.AuthorizeID(serverID)))
		got, err := checkHealth(address, creds)
		if err != nil {
			t.Fatal(err)
		}
		if got != grpc_health_v1.HealthCheckResponse_SERVING {
			t.Fatalf("got %s, want SERVING", got)
		}
	})

	t.Run("unauthorized client", func(t *testing.T) {
		creds := credentials.NewTLS(tlsconfig.MTLSClientConfig(ca.NewSVID(t, otherID), bundle, tlsconfig.AuthorizeID(serverID)))
		if _, err := checkHealth(address, creds); err == nil {
			t.Fatal("expected the client to be rejected")
		}
	})

	t.Run("no client certificate", func(t *testing.T) {
		creds := credentials.NewTLS(tlsconfig.TLSClientConfig(bundle, tlsconfig.AuthorizeID(serverID)))
		if _, err := checkHealth(address, creds); err == nil {
			t.Fatal("expected the client to be rejected")
		}
	})

	t.Run("foreign trust domain", func(t *testing.T) {
		evil := spiffetest.NewCA(t)
		foreign := spiffeid.RequireFromString("spiffe://evil.org/ns/default/sa/attacker")
		creds := credentials.NewTLS(tlsconfig.MTLSClientConfig(evil.NewSVID(t, foreign), bundle, tlsconfig.AuthorizeID(serverID)))
		if _, err := checkHealth(address, creds); err == nil {
			t.Fatal("expected the client to be rejected")
		}
	})
}

func TestGrpcSPIFFEMTLS_Unready(t *testing.T) {
	ca := spiffetest.NewCA(t)
	socket := spiffetest.SocketPath(t)
	spiffetest.StartWorkloadAPI(t, socket, ca, ca.NewSVID(t, serverID))
	source := spiffe.NewSource("unix://"+socket, nil, zap.NewNop(), spiffetest.FastBackoff())
	t.Cleanup(func() { _ = source.Close() })

	s := NewServer(&Config{ServiceName: "podinfo", TLS: true, Unready: true}, zap.NewNop())
	s.SetSPIFFESource(source)
	_, healthSrv := startTLSServer(t, s)

	if _, err := source.Wait(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	if got := serviceStatus(t, healthSrv); got != grpc_health_v1.HealthCheckResponse_NOT_SERVING {
		t.Fatalf("--unready: got %s, want NOT_SERVING", got)
	}
}

// echoedSPIFFEIDs returns the x-spiffe-id values echoed by the Header and Pod
// services to a client that sends a forged x-spiffe-id.
func echoedSPIFFEIDs(t *testing.T, address string, creds credentials.TransportCredentials) (header, pod []string) {
	t.Helper()
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(creds))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), waitTimeout)
	t.Cleanup(cancel)
	ctx = metadata.AppendToOutgoingContext(ctx, "x-spiffe-id", "spiffe://example.org/forged")

	headerRes, err := headerv1.NewHeaderServiceClient(conn).Header(ctx, &headerv1.HeaderRequest{})
	if err != nil {
		t.Fatalf("Header: %v", err)
	}
	for _, h := range headerRes.GetHeaders() {
		if v, ok := strings.CutPrefix(h, "x-spiffe-id="); ok {
			header = append(header, v)
		}
	}

	podRes, err := podv1.NewPodServiceClient(conn).Pod(ctx, &podv1.PodRequest{})
	if err != nil {
		t.Fatalf("Pod: %v", err)
	}
	return header, podRes.GetHeaders()["x-spiffe-id"].GetValues()
}

func TestGrpcSPIFFEMTLS_PeerID(t *testing.T) {
	ca := spiffetest.NewCA(t)
	socket := spiffetest.SocketPath(t)
	spiffetest.StartWorkloadAPI(t, socket, ca, ca.NewSVID(t, serverID))
	source := spiffe.NewSource("unix://"+socket, []spiffeid.ID{clientID}, zap.NewNop(), spiffetest.FastBackoff())
	t.Cleanup(func() { _ = source.Close() })

	s := NewServer(&Config{ServiceName: "podinfo", TLS: true}, zap.NewNop())
	s.SetSPIFFESource(source)
	address, _ := startTLSServer(t, s)
	if _, err := source.Wait(); err != nil {
		t.Fatal(err)
	}

	creds := credentials.NewTLS(tlsconfig.MTLSClientConfig(ca.NewSVID(t, clientID), ca.Bundle(serverID.TrustDomain()), tlsconfig.AuthorizeID(serverID)))
	header, pod := echoedSPIFFEIDs(t, address, creds)

	want := []string{clientID.String()}
	if !slices.Equal(header, want) {
		t.Errorf("Header x-spiffe-id = %v, want %v", header, want)
	}
	if !slices.Equal(pod, want) {
		t.Errorf("Pod x-spiffe-id = %v, want %v", pod, want)
	}
}

func TestGrpcTLS_NoPeerID(t *testing.T) {
	dir := t.TempDir()
	cert := writeCert(t, dir)

	kp, err := keypair.New(dir, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = kp.Close() })

	s := NewServer(&Config{ServiceName: "podinfo", TLS: true}, zap.NewNop())
	s.SetKeyPair(kp)
	address, _ := startTLSServer(t, s)

	roots := x509.NewCertPool()
	roots.AddCert(cert)
	header, pod := echoedSPIFFEIDs(t, address, credentials.NewTLS(&tls.Config{RootCAs: roots}))
	if len(header) != 0 {
		t.Errorf("Header x-spiffe-id = %v, want none", header)
	}
	if len(pod) != 0 {
		t.Errorf("Pod x-spiffe-id = %v, want none", pod)
	}
}
