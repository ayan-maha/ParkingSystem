package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCORS(t *testing.T) {
	called := false
	handler := withCORS("http://localhost:8091", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodOptions, "/v1/spaces/P01", nil)
	req.Header.Set("Origin", "http://localhost:8091")
	req.Header.Set("Access-Control-Request-Method", "PUT")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent || called || !strings.Contains(w.Header().Get("Access-Control-Allow-Methods"), "PUT") || w.Header().Get("Access-Control-Allow-Origin") != "http://localhost:8091" {
		t.Fatalf("invalid preflight: status=%d headers=%v handlerCalled=%v", w.Code, w.Header(), called)
	}
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPut, "/v1/spaces/P01", nil))
	if !called {
		t.Fatal("actual request did not reach the handler")
	}
}
