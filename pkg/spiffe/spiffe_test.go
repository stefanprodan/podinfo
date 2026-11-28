package spiffe

import (
	"errors"
	"testing"
	"time"

	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/stefanprodan/podinfo/pkg/spiffe/spiffetest"
)

var (
	serverID  = spiffeid.RequireFromString("spiffe://example.org/ns/default/sa/podinfo")
	clientID  = spiffeid.RequireFromString("spiffe://example.org/ns/flux-system/sa/source-watcher")
	otherID   = spiffeid.RequireFromString("spiffe://example.org/ns/default/sa/other")
	foreignID = spiffeid.RequireFromString("spiffe://evil.org/ns/default/sa/attacker")
)

func TestParseIDs(t *testing.T) {
	ids, err := ParseIDs("--spiffe-authorized-ids", []string{"spiffe://example.org/a", " ", " spiffe://example.org/b "})
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 2 || ids[1].String() != "spiffe://example.org/b" {
		t.Fatalf("unexpected IDs: %v", ids)
	}

	for _, bad := range []string{"not-an-id", "https://example.org/a", "spiffe://"} {
		if _, err := ParseIDs("--spiffe-authorized-ids", []string{bad}); err == nil {
			t.Errorf("ParseIDs(%q) returned no error", bad)
		}
	}
}

func TestAuthorizer(t *testing.T) {
	cases := []struct {
		name    string
		ids     []spiffeid.ID
		peer    spiffeid.ID
		allowed bool
	}{
		{name: "listed ID", ids: []spiffeid.ID{clientID}, peer: clientID, allowed: true},
		{name: "unlisted ID", ids: []spiffeid.ID{clientID}, peer: otherID, allowed: false},
		{name: "same trust domain", peer: otherID, allowed: true},
		{name: "foreign trust domain", peer: foreignID, allowed: false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := Authorizer(c.ids, serverID)(c.peer, nil)
			if c.allowed && err != nil {
				t.Fatalf("expected %s to be allowed: %v", c.peer, err)
			}
			if !c.allowed && err == nil {
				t.Fatalf("expected %s to be rejected", c.peer)
			}
		})
	}
}

func TestSourceLifecycle(t *testing.T) {
	ca := spiffetest.NewCA(t)
	socket := spiffetest.SocketPath(t)
	src := NewSource("unix://"+socket, nil, zap.NewNop(), spiffetest.FastBackoff())
	src.Start()

	if _, err := src.GetX509SVID(); !errors.Is(err, errNotIssued) {
		t.Fatalf("GetX509SVID before the SVID: got %v, want %v", err, errNotIssued)
	}
	if _, err := src.GetX509BundleForTrustDomain(serverID.TrustDomain()); !errors.Is(err, errNotIssued) {
		t.Fatalf("GetX509BundleForTrustDomain before the SVID: got %v, want %v", err, errNotIssued)
	}

	spiffetest.StartWorkloadAPI(t, socket, ca, ca.NewSVID(t, serverID))
	id, err := src.Wait()
	if err != nil {
		t.Fatal(err)
	}
	if id != serverID {
		t.Fatalf("Wait returned %s, want %s", id, serverID)
	}
	if _, err := src.GetX509BundleForTrustDomain(serverID.TrustDomain()); err != nil {
		t.Fatalf("GetX509BundleForTrustDomain after the SVID: %v", err)
	}

	if err := src.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := src.GetX509SVID(); !errors.Is(err, errClosed) {
		t.Fatalf("GetX509SVID after Close: got %v, want %v", err, errClosed)
	}
}

func TestSourceStopWhileWaiting(t *testing.T) {
	src := NewSource("unix://"+spiffetest.SocketPath(t), nil, zap.NewNop(), spiffetest.FastBackoff())
	src.Start()
	time.Sleep(100 * time.Millisecond)

	done := make(chan error, 1)
	go func() {
		_, err := src.Wait()
		done <- err
	}()
	src.Stop()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("Wait returned no error after Stop")
		}
	case <-time.After(30 * time.Second):
		t.Fatal("Wait hung after Stop")
	}
	if err := src.Close(); err != nil {
		t.Fatalf("Close returned an error: %v", err)
	}
}

func TestSourceCloseNotStarted(t *testing.T) {
	src := NewSource("unix:///tmp/agent.sock", nil, zap.NewNop())
	if err := src.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestLogger(t *testing.T) {
	core, logs := observer.New(zap.DebugLevel)
	l := &logger{logger: zap.New(core)}

	l.Errorf("Failed to watch the Workload API: %v", errors.New("no identity issued"))
	l.ready.Store(true)
	l.Errorf("Failed to watch the Workload API: %v", errors.New("connection reset"))

	entries := logs.All()
	if len(entries) != 2 {
		t.Fatalf("got %d log entries, want 2", len(entries))
	}
	if entries[0].Level != zap.InfoLevel {
		t.Errorf("error before the first SVID logged at %s, want info", entries[0].Level)
	}
	if entries[1].Level != zap.WarnLevel {
		t.Errorf("error after the first SVID logged at %s, want warn", entries[1].Level)
	}
}
