package oci

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

// Create a tar.gz file for testing
func createTarGzFile(t *testing.T, dest string, content map[string]string) {
	var buf bytes.Buffer
	gzipWriter := gzip.NewWriter(&buf)
	tarWriter := tar.NewWriter(gzipWriter)

	for filename, data := range content {
		if err := tarWriter.WriteHeader(&tar.Header{
			Name: filename,
			Mode: 0600,
			Size: int64(len(data)),
		}); err != nil {
			t.Fatalf("failed to write tar header for %s: %v", filename, err)
		}
		if _, err := tarWriter.Write([]byte(data)); err != nil {
			t.Fatalf("failed to write data for %s: %v", filename, err)
		}
	}

	if err := tarWriter.Close(); err != nil {
		t.Fatalf("failed to close tar writer: %v", err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatalf("failed to close gzip writer: %v", err)
	}

	if err := ioutil.WriteFile(dest, buf.Bytes(), 0644); err != nil {
		t.Fatalf("failed to write tar.gz file: %v", err)
	}
}

// Test extractTarGz method
func TestExtractTarGz(t *testing.T) {
	dest := t.TempDir() // Temporary directory for extraction
	tarGzFile := filepath.Join(dest, "test.tar.gz")

	content := map[string]string{
		"file1.txt": "This is the content of file1.",
		"file2.txt": "This is the content of file2.",
	}

	createTarGzFile(t, tarGzFile, content)

	controller := &Controller{}

	// Use os.Open to pass an io.Reader that represents the tar.gz file
	file, err := os.Open(tarGzFile)
	if err != nil {
		t.Fatalf("failed to open tar.gz file: %v", err)
	}
	defer file.Close()

	// Extract the tar.gz file using the extractTarGz method
	if err := controller.extractTarGz(file, dest); err != nil {
		t.Fatalf("failed to extract tar.gz file: %v", err)
	}

	// Verify the contents of the extracted files
	for filename, expectedContent := range content {
		filePath := filepath.Join(dest, filename)
		actualContent, err := ioutil.ReadFile(filePath)
		if err != nil {
			t.Fatalf("failed to read extracted file %s: %v", filename, err)
		}
		if string(actualContent) != expectedContent {
			t.Errorf("content mismatch for %s: expected %s, got %s", filename, expectedContent, string(actualContent))
		}
	}
}
