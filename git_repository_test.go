package main

import (
	"context"
	"testing"
)

func TestGitRepository(t *testing.T) {
	giturl := "https://github.com/divakarmanoj/go-remote-config-server.git"
	repo, err := NewGitRepository(giturl, "test.yaml")
	if err != nil {
		t.Error(err)
	}
	data, err := repo.getData(context.Background())
	if err != nil {
		t.Error(err)
	}
	if data == "" {
		t.Error("data is empty")
	}

	data, err = repo.getData(context.Background())
	if err != nil {
		t.Error(err)
	}
	if data == "" {
		t.Error("data is empty")
	}

	// test GetUrl()
	url := repo.getUrl()
	if url.String() != giturl {
		t.Errorf("expected %q, got %q", giturl, url.String())
	}

	// test GetPath()
	path := repo.getPath()
	if path != "test.yaml" {
		t.Errorf("expected %q, got %q", "test.yaml", path)
	}

}
