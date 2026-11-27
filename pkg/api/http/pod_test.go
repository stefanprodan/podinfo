package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
)

func TestPodHandler(t *testing.T) {
	t.Setenv("PODINFO_TEST_PLAIN", "plain")
	t.Setenv("PODINFO_TEST_PASSWORD", "s3cret")

	req := httptest.NewRequest("GET", "/api/pod", nil)
	req.Header.Set("X-Test", "podinfo")
	req.Header.Set(spiffeIDHeader, "spiffe://example.org/forged")
	req.Header.Set("Authorization", "Bearer s3cret")

	rr := httptest.NewRecorder()
	http.HandlerFunc(NewMockServer().podHandler).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	var pod PodResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &pod); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if pod.Hostname != "localhost" {
		t.Errorf("hostname = %q, want %q", pod.Hostname, "localhost")
	}
	if pod.Runtime == "" || pod.NumCPU == "" {
		t.Errorf("runtime fields missing: %+v", pod)
	}
	if got := pod.Env["PODINFO_TEST_PLAIN"]; got != "plain" {
		t.Errorf("env PODINFO_TEST_PLAIN = %q, want %q", got, "plain")
	}
	if got := pod.Env["PODINFO_TEST_PASSWORD"]; got != "[REDACTED]" {
		t.Errorf("env PODINFO_TEST_PASSWORD = %q, want it masked", got)
	}
	if got := pod.Headers["X-Test"]; !slices.Equal(got, []string{"podinfo"}) {
		t.Errorf("header X-Test = %v, want [podinfo]", got)
	}
	if got := pod.Headers["Authorization"]; !slices.Equal(got, []string{"[REDACTED]"}) {
		t.Errorf("header Authorization = %v, want it masked", got)
	}
	if got, ok := pod.Headers[spiffeIDHeader]; ok {
		t.Errorf("client-supplied %s echoed: %v", spiffeIDHeader, got)
	}
}
