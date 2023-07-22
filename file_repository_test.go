package main

import (
	"context"
	"os"
	"testing"
)

func TestFileRepository(t *testing.T) {
	// Create a temporary file for testing
	tmpfile, err := os.CreateTemp("", "example")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name()) // clean up
	testData := `---
name: John Doe
age: 30
email: johndoe@example.com
is_employee: true
address:
  city: New York
  country: USA
  zip_code: "10001"
hobbies:
  - Reading
  - Cooking
  - Hiking
  - Swimming
  - Coding
`
	_, err = tmpfile.Write([]byte(testData))
	if err != nil {
		t.Fatal(err)
	}
	// Close the file
	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}
	// happy path
	repo, err := NewFileRepository(tmpfile.Name())
	if err != nil {
		t.Fatal(err)
	}
	data, err := repo.getData(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if data != testData {
		t.Fatal("data does not match")
	}

	// test GetUrl()
	url := repo.getUrl()
	expectedURL, err := filePathToURL(tmpfile.Name())
	if err != nil {
		t.Fatal(err)
	}
	expected := expectedURL.String()
	if url.String() != expected {
		t.Errorf("expected %q, got %q", expected, url.String())
	}

	// test GetPath()
	path := repo.getPath()
	if path != tmpfile.Name() {
		t.Errorf("expected %q, got %q", tmpfile.Name(), path)
	}

	// sad path
	repo, err = NewFileRepository("/tmp/does-not-exist")
	data, err = repo.getData(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}

}
