package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMainHandler(t *testing.T) {
	// Test the "GET" case
	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	MainHandler(rr, req)
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	// Test the "HEAD" case
	req, err = http.NewRequest("HEAD", "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	rr = httptest.NewRecorder()
	MainHandler(rr, req)
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}
}
