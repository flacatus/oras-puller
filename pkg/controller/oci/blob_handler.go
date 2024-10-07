package oci

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	extractTimeout = 1 * time.Minute
)

// Extracts tar.gz files to a specified destination.
// It takes an io.Reader for the gzip stream and the destination path.
func (c *Controller) extractTarGz(gzipStream io.Reader, dest string) error {
	uncompressedStream, err := gzip.NewReader(gzipStream)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer uncompressedStream.Close()

	tarReader := tar.NewReader(uncompressedStream)

	// Iterate through the tar entries
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar header: %w", err)
		}

		destPath := filepath.Join(dest, header.Name)
		if err := c.handleTarEntry(header, tarReader, destPath); err != nil {
			return err
		}
	}
	return nil
}

// Handles individual entries in the tar archive.
// It creates directories or files as specified in the tar header.
func (c *Controller) handleTarEntry(header *tar.Header, tarReader *tar.Reader, destPath string) error {
	switch header.Typeflag {
	case tar.TypeDir:
		return os.MkdirAll(destPath, 0755)
	case tar.TypeReg:
		if _, err := os.Stat(destPath); err == nil {
			return nil
		}
		return c.createFileFromTar(tarReader, destPath)
	default:
		return fmt.Errorf("unsupported tar entry: %c", header.Typeflag)
	}
}

// Creates a file from the tar reader.
// It writes the contents of the tar entry to a newly created file.
func (c *Controller) createFileFromTar(tarReader *tar.Reader, destPath string) error {
	outFile, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", destPath, err)
	}
	defer outFile.Close()

	// Copy contents from the tar reader to the file
	if _, err := io.Copy(outFile, tarReader); err != nil {
		return fmt.Errorf("failed to write file %s: %w", destPath, err)
	}
	return nil
}

// Handles the extraction of individual blobs.
// It manages concurrency with WaitGroup and semaphore for blob processing.
func (c *Controller) HandleBlob(blobPath, outputDir string, wg *sync.WaitGroup, errors chan<- error, sem chan struct{}) {
	defer wg.Done()
	sem <- struct{}{}
	defer func() { <-sem }()

	// Process the blob file for extraction
	if err := c.processBlob(blobPath, outputDir); err != nil {
		errors <- err
	}
}

// Processes the blob file for extraction.
// It checks for file existence, size, and identifies if it's a tar.gz blob.
func (c *Controller) processBlob(blobPath, outputDir string) error {
	fileInfo, err := os.Stat(blobPath)
	if err != nil {
		return fmt.Errorf("failed to stat blob %s: %w", blobPath, err)
	}

	if fileInfo.Size() == 0 {
		return fmt.Errorf("blob %s is empty, skipping", blobPath)
	}

	file, err := os.Open(blobPath)
	if err != nil {
		return fmt.Errorf("failed to open blob %s: %w", blobPath, err)
	}
	defer file.Close()

	if isTarGzBlob(blobPath, file) {
		return c.extractBlob(blobPath, file, outputDir)
	}

	return nil
}

// Determines if the blob is a tar.gz file.
// It checks the file extension and the header bytes for gzip format.
func isTarGzBlob(blobPath string, file *os.File) bool {
	buf := make([]byte, 2)
	if _, err := file.Read(buf); err != nil {
		return false
	}
	file.Seek(0, 0)
	return strings.HasSuffix(blobPath, ".tar.gz") || (len(buf) == 2 && buf[0] == 0x1F && buf[1] == 0x8B)
}

// Extracts a tar.gz blob to the specified output directory.
// It handles timeouts during extraction.
func (c *Controller) extractBlob(blobPath string, file *os.File, outputDir string) error {
	ctx, cancel := context.WithTimeout(context.Background(), extractTimeout)
	defer cancel()

	extractErr := make(chan error, 1)
	go func() {
		extractErr <- c.extractTarGz(file, outputDir)
	}()

	select {
	case err := <-extractErr:
		if err != nil {
			return fmt.Errorf("failed to extract tar.gz blob %s: %w", blobPath, err)
		}
	case <-ctx.Done():
		return fmt.Errorf("timeout while extracting blob %s", blobPath)
	}

	return nil
}
