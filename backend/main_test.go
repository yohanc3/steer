package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"yohanc3/steer/teller"
)

func TestHealth(t *testing.T) {
	handler := newAPIServer(http.NotFoundHandler(), teller.TellerService{})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
}
