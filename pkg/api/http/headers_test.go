package http

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
)

func TestEchoHeadersHandler(t *testing.T) {
	req, err := http.NewRequest("GET", "/api/headers", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Test", "testing")
	rr := httptest.NewRecorder()
	srv := NewMockServer()
	handler := http.HandlerFunc(srv.echoHeadersHandler)

	handler.ServeHTTP(rr, req)

	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	// Check the response body is what we expect.
	expected := "testing"
	r := regexp.MustCompile(fmt.Sprintf("(?m:%s)", expected))
	if !r.MatchString(rr.Body.String()) {
		t.Fatalf("handler returned unexpected body:\ngot \n%v \nwant \n%s",
			rr.Body.String(), expected)
	}
}

func TestEchoHeadersHandlerMasksCredentials(t *testing.T) {
	req := httptest.NewRequest("GET", "/headers", nil)
	req.Header.Set("Authorization", "Bearer s3cret")
	req.Header.Set("Cookie", "session=s3cret")

	rr := httptest.NewRecorder()
	http.HandlerFunc(NewMockServer().echoHeadersHandler).ServeHTTP(rr, req)

	if body := rr.Body.String(); regexp.MustCompile("s3cret").MatchString(body) {
		t.Fatalf("credential echoed:\n%s", body)
	}
}
