// Copyright 2021 Stefan Prodan.
// SPDX-License-Identifier: Apache-2.0

package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/golang-jwt/jwt/v4"
)

func TestTokenHandler(t *testing.T) {
	req, err := http.NewRequest(http.MethodPost, "/token", strings.NewReader("test-user"))
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	srv := NewMockServer()
	handler := http.HandlerFunc(srv.tokenGenerateHandler)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	var token TokenResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &token); err != nil {
		t.Fatal(err)
	}
	if token.Token == "" {
		t.Error("handler returned no token")
	}
}

func TestTokenGenerateHandler_EmptyBody(t *testing.T) {
	srv := NewMockServer()
	srv.config.JWTSecret = "test-secret"

	req, _ := http.NewRequest(http.MethodPost, "/token", strings.NewReader(""))
	rr := httptest.NewRecorder()
	http.HandlerFunc(srv.tokenGenerateHandler).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d", rr.Code, http.StatusOK)
	}

	var result TokenResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Token == "" {
		t.Error("expected non-empty token")
	}
}

func TestTokenValidateHandler(t *testing.T) {
	srv := NewMockServer()
	srv.config.JWTSecret = "test-secret"

	genReq, _ := http.NewRequest(http.MethodPost, "/token", strings.NewReader("test-user"))
	genRR := httptest.NewRecorder()
	http.HandlerFunc(srv.tokenGenerateHandler).ServeHTTP(genRR, genReq)

	var tokenResp TokenResponse
	if err := json.Unmarshal(genRR.Body.Bytes(), &tokenResp); err != nil {
		t.Fatal(err)
	}

	t.Run("valid token", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/token/validate", nil)
		req.Header.Set("Authorization", "Bearer "+tokenResp.Token)
		rr := httptest.NewRecorder()
		http.HandlerFunc(srv.tokenValidateHandler).ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("got status %d, want %d", rr.Code, http.StatusOK)
		}
		var result TokenValidationResponse
		if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if result.TokenName != "test-user" {
			t.Errorf("token_name = %q, want %q", result.TokenName, "test-user")
		}
	})

	t.Run("missing authorization header", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/token/validate", nil)
		rr := httptest.NewRecorder()
		http.HandlerFunc(srv.tokenValidateHandler).ServeHTTP(rr, req)

		if !strings.Contains(rr.Body.String(), "authorization bearer header required") {
			t.Errorf("unexpected body: %s", rr.Body.String())
		}
	})

	t.Run("malformed authorization header", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/token/validate", nil)
		req.Header.Set("Authorization", "InvalidFormat")
		rr := httptest.NewRecorder()
		http.HandlerFunc(srv.tokenValidateHandler).ServeHTTP(rr, req)

		if !strings.Contains(rr.Body.String(), "authorization bearer header required") {
			t.Errorf("unexpected body: %s", rr.Body.String())
		}
	})

	t.Run("invalid token", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/token/validate", nil)
		req.Header.Set("Authorization", "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJuYW1lIjoiZmFrZSIsImlzcyI6InBvZGluZm8iLCJleHAiOjk5OTk5OTk5OTl9.invalidsig")
		rr := httptest.NewRecorder()
		http.HandlerFunc(srv.tokenValidateHandler).ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("got status %d, want %d", rr.Code, http.StatusUnauthorized)
		}
	})

	t.Run("token without expiry", func(t *testing.T) {
		unexpiring, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwtCustomClaims{
			Name:   "test-user",
			Issuer: "podinfo",
		}).SignedString([]byte(srv.config.JWTSecret))
		if err != nil {
			t.Fatal(err)
		}
		req, _ := http.NewRequest(http.MethodGet, "/token/validate", nil)
		req.Header.Set("Authorization", "Bearer "+unexpiring)
		rr := httptest.NewRecorder()
		http.HandlerFunc(srv.tokenValidateHandler).ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("got status %d, want %d", rr.Code, http.StatusUnauthorized)
		}
	})

	t.Run("wrong signing secret", func(t *testing.T) {
		otherSrv := NewMockServer()
		otherSrv.config.JWTSecret = "different-secret"

		req, _ := http.NewRequest(http.MethodGet, "/token/validate", nil)
		req.Header.Set("Authorization", "Bearer "+tokenResp.Token)
		rr := httptest.NewRecorder()
		http.HandlerFunc(otherSrv.tokenValidateHandler).ServeHTTP(rr, req)

		if !strings.Contains(rr.Body.String(), "signature is invalid") {
			t.Errorf("expected signature error, got: %s", rr.Body.String())
		}
	})
}
