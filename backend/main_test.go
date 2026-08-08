package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealth(t *testing.T) {
	handler:=newHandler(http.NotFoundHandler(),func(w http.ResponseWriter,_ *http.Request){w.WriteHeader(http.StatusNoContent)})
	response:=httptest.NewRecorder();handler.ServeHTTP(response,httptest.NewRequest(http.MethodGet,"/healthz",nil));if response.Code!=http.StatusOK{t.Fatalf("status = %d",response.Code)}
}
