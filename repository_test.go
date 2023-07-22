package main

import (
	"testing"
)

func TestNewRepository(t *testing.T) {
	*path = "testdata/test.yml"
	*URL = "https://raw.githubusercontent.com/divakarmanoj/go-scaffolding/main/testdata/test.yml"
	// Test the "fs" case
	fsRepo, err := NewRepository("fs")
	if err != nil {
		t.Fatal(err)
	}
	if fsRepo.getType() != "fs" {
		t.Errorf("expected type %q, got %q", "fs", fsRepo.getType())
	}

	// Test the "git" case
	gitRepo, err := NewRepository("git")
	if err != nil {
		t.Fatal(err)
	}
	if gitRepo.getType() != "git" {
		t.Errorf("expected type %q, got %q", "git", gitRepo.getType())
	}

	// Test the "http" case
	httpRepo, err := NewRepository("http")
	if err != nil {
		t.Fatal(err)
	}
	if httpRepo.getType() != "http" {
		t.Errorf("expected type %q, got %q", "http", httpRepo.getType())
	}

	// Test the default case
	defaultRepo, err := NewRepository("invalid")
	if err != nil {
		t.Fatal(err)
	}
	if defaultRepo.getType() != "fs" {
		t.Errorf("expected type %q, got %q", "fs", defaultRepo.getType())
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
