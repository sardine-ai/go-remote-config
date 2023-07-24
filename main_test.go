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

func TestNewRepository(t *testing.T) {
	*path = "testdata/test.yml"
	*URL = "https://raw.githubusercontent.com/divakarmanoj/go-scaffolding/main/testdata/test.yml"
	// Test the "fs" case
	fsRepo, err := NewRepository("fs")
	if err != nil {
		t.Fatal(err)
	}
	if fsRepo.GetType() != "fs" {
		t.Errorf("expected type %q, got %q", "fs", fsRepo.GetType())
	}

	// Test the "git" case
	gitRepo, err := NewRepository("git")
	if err != nil {
		t.Fatal(err)
	}
	if gitRepo.GetType() != "git" {
		t.Errorf("expected type %q, got %q", "git", gitRepo.GetType())
	}

	// Test the "http" case
	httpRepo, err := NewRepository("http")
	if err != nil {
		t.Fatal(err)
	}
	if httpRepo.GetType() != "http" {
		t.Errorf("expected type %q, got %q", "http", httpRepo.GetType())
	}

	// Test the default case
	defaultRepo, err := NewRepository("invalid")
	if err != nil {
		t.Fatal(err)
	}
	if defaultRepo.GetType() != "fs" {
		t.Errorf("expected type %q, got %q", "fs", defaultRepo.GetType())
	}

	*URL = ""
	// Test the "git" case with missing URL and path
	_, err = NewRepository("git")
	if err == nil {
		t.Error("expected error, got nil")
	}

	// Test the "http" case with missing URL
	_, err = NewRepository("http")
	if err == nil {
		t.Error("expected error, got nil")
	}
}
