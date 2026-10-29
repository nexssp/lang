package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteNewFileCreatesOutput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "generated.nflow")
	if err := writeNewFile(path, "@schema Input {}\n"); err != nil {
		t.Fatalf("writeNewFile() error = %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(got) != "@schema Input {}\n" {
		t.Fatalf("file contents = %q", got)
	}
}

func TestWriteNewFileDoesNotOverwrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "generated.nflow")
	const original = "keep this file\n"
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatalf("seed existing output: %v", err)
	}
	if err := writeNewFile(path, "replacement\n"); err == nil {
		t.Fatal("writeNewFile() unexpectedly overwrote an existing file")
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(got) != original {
		t.Fatalf("existing file changed: got %q, want %q", got, original)
	}
}
