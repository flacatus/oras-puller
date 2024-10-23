package oci

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/oci"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/credentials"
	"oras.land/oras-go/v2/registry/remote/retry"
)

// Constants for configurable settings
const (
	blobTimeout      = 2 * time.Minute
	tagDaysThreshold = 4
)

// Processes individual tags from a given repository
func (c *Controller) ProcessTag(repo, tag, creationDate string) error {

	if err := c.validateCreationDate(creationDate); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), blobTimeout)
	defer cancel()

	repoRemote, err := c.setupRemoteRepository(repo)
	if err != nil {
		return err
	}

	if err := c.copyTagManifest(ctx, repoRemote, tag, c.Store); err != nil {
		return err
	}

	outputDir := c.createOutputDirectory(repo, creationDate, tag)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory %s: %w", outputDir, err)
	}

	return c.processBlobs(outputDir)
}

// Validates the creation date of the tag
func (c *Controller) validateCreationDate(creationDate string) error {
	parsedDate, err := time.Parse(time.RFC1123, creationDate)
	if err != nil {
		return fmt.Errorf("failed to parse creation date %s: %w", creationDate, err)
	}

	if time.Since(parsedDate) > time.Duration(tagDaysThreshold)*24*time.Hour {
		return nil
	}

	return nil
}

// Sets up the remote repository for the given repo name
func (c *Controller) setupRemoteRepository(repo string) (*remote.Repository, error) {
	repoRemote, err := remote.NewRepository("quay.io/" + repo)
	if err != nil {
		return nil, fmt.Errorf("failed to set up remote repository %s: %w", repo, err)
	}

	credStore, err := credentials.NewStoreFromDocker(credentials.StoreOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create credential store: %w", err)
	}

	repoRemote.Client = &auth.Client{
		Client:     retry.DefaultClient,
		Cache:      auth.NewCache(),
		Credential: credentials.Credential(credStore),
	}

	return repoRemote, nil
}

// Copies the tag manifest from the remote repository to the local OCI store
func (c *Controller) copyTagManifest(ctx context.Context, repoRemote *remote.Repository, tag string, store *oci.Store) error {
	if _, err := oras.Copy(ctx, repoRemote, tag, store, tag, oras.DefaultCopyOptions); err != nil {
		return fmt.Errorf("failed to copy manifest for tag %s: %w", tag, err)
	}
	return nil
}

// Creates the output directory for the blobs
func (c *Controller) createOutputDirectory(repo, creationDate, tag string) string {
	parsedDate, _ := time.Parse(time.RFC1123, creationDate)
	return filepath.Join(c.OutputDir, repo, parsedDate.Format("2006-01-02"), tag)
}

// Processes the blobs by handling individual blob files
func (c *Controller) processBlobs(outputDir string) error {
	var wg sync.WaitGroup
	errors := make(chan error, 10)
	sem := make(chan struct{}, 10)

	entries, err := os.ReadDir(c.BlobDir)
	if err != nil {
		return fmt.Errorf("failed to read blob directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		wg.Add(1)
		go c.HandleBlob(filepath.Join(c.BlobDir, entry.Name()), outputDir, &wg, errors, sem)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		log.Println("Error:", err)
	}

	return nil
}
