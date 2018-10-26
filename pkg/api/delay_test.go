package api

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
)

func TestDelayHandler(t *testing.T) {
	req, err := http.NewRequest("GET", "/delay/0", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	srv := NewMockServer()

	srv.router.HandleFunc("/delay/{wait}", srv.delayHandler)
	srv.router.ServeHTTP(rr, req)

	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	// Check the response body is what we expect.
	expected := ".*delay.*0.*"
	r := regexp.MustCompile(expected)
	if !r.MatchString(rr.Body.String()) {
		t.Fatalf("handler returned unexpected body:\ngot \n%v \nwant \n%s",
			rr.Body.String(), expected)
	}
}
