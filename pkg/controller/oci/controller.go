package oci

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/oci"
)

// Controller orchestrates operations on OCI repositories.
// It holds the configuration for output and blob directories.
type Controller struct {
	// OutputDir is the directory where output files are stored.
	OutputDir string

	// BlobDir is the directory where blob files are stored.
	BlobDir string

	// OCIStorePath is the path to the local OCI store.
	OCIStorePath string

	// Store is the OCI store instance.
	Store *oci.Store
}

// NewController initializes a new Controller instance with the specified output and OCI store path.
func NewController(outputDir string, OCIStorePath string) (*Controller, error) {
	store, err := oci.New(OCIStorePath)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize OCI store at path %s: %w", OCIStorePath, err)
	}

	return &Controller{
		OutputDir:    outputDir,
		BlobDir:      OCIStorePath + "/blobs/sha256/",
		OCIStorePath: OCIStorePath,
		Store:        store,
	}, nil
}

// FetchOCIContainerAnnotations fetches the OCI container annotations for a given repository and tag.
// It retrieves the descriptor content by copying the tag manifest to the OCI store and unmarshaling it into a Descriptor struct.
func (c *Controller) FetchOCIContainerAnnotations(repo, tag string) (*v1.Descriptor, error) {
	ctx := context.Background()

	repoRemote, err := c.setupRemoteRepository(repo)
	if err != nil {
		return nil, fmt.Errorf("failed to set up remote repository for %s: %w", repo, err)
	}

	if err := c.copyTagManifest(ctx, repoRemote, tag, c.Store); err != nil {
		return nil, fmt.Errorf("failed to copy manifest for tag %s: %w", tag, err)
	}

	_, descriptorBytes, err := oras.FetchBytes(ctx, c.Store, tag, oras.FetchBytesOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch descriptor bytes for tag %s: %w", tag, err)
	}

	var descriptor v1.Descriptor
	if err := json.Unmarshal(descriptorBytes, &descriptor); err != nil {
		return nil, fmt.Errorf("failed to unmarshal descriptor content: %w", err)
	}

	return &descriptor, nil
}

// ProcessRepositories processes multiple repositories concurrently.
// It fetches and processes tags for each repository, limiting concurrency to avoid overwhelming system resources.
// Returns a slice of errors encountered during the processing of repositories.
func (c *Controller) ProcessRepositories(repositories []string) []error {
	var wg sync.WaitGroup
	errorsChan := make(chan error, len(repositories))

	sem := make(chan struct{}, 10)

	// Loop over each repository and process it concurrently.
	for _, repo := range repositories {
		wg.Add(1)

		go func(repo string) {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			if err := c.processRepository(repo); err != nil {
				errorsChan <- fmt.Errorf("repository %s: %w", repo, err)
			}
		}(repo)
	}

	wg.Wait()
	close(errorsChan)

	var errors []error
	for err := range errorsChan {
		errors = append(errors, err)
	}

	return errors
}

// processRepository fetches and processes tags for a specific repository.
// It returns an error if any issues occur while fetching or processing tags.
func (c *Controller) processRepository(repo string) error {
	// Fetch tags for the specified repository.
	tags, err := c.FetchTags(repo)
	if err != nil {
		return fmt.Errorf("failed to fetch tags for repository %s: %w", repo, err)
	}

	// Process each tag within the repository.
	for _, tagInfo := range tags {
		if err := c.ProcessTag(repo, tagInfo.Name, tagInfo.LastModified); err != nil {
			return fmt.Errorf("failed to process tag %s in repository %s: %w", tagInfo.Name, repo, err)
		}
	}

	return nil
}
