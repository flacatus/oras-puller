package oci

import (
	"fmt"
	"sync"
)

// Controller orchestrates operations on repositories.
// It holds the configuration for output directories.
type Controller struct {
	// Directory to store output files
	OutputDir string

	// Directory where blob files are stored
	BlobDir string

	// Path to the OCI store
	OCIStorePath string
}

// NewController initializes a new Controller instance with the specified output and blob directories.
func NewController(outputDir string, OCIStorePath string) *Controller {
	return &Controller{
		OutputDir:    outputDir,
		BlobDir:      OCIStorePath + "/blobs/sha256/",
		OCIStorePath: OCIStorePath,
	}
}

// ProcessRepositories orchestrates the processing of multiple repositories concurrently.
// It fetches tags for each repository and processes them.
// Returns a slice of encountered errors during processing.
func (c *Controller) ProcessRepositories(repositories []string) []error {
	var wg sync.WaitGroup
	// Buffered channel to collect errors
	errorsChan := make(chan error, len(repositories))

	// Semaphore to control concurrency
	sem := make(chan struct{}, 10)

	for _, repo := range repositories {
		wg.Add(1)

		go func(repo string) {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			if err := c.processRepository(repo); err != nil {
				errorsChan <- err
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

// processRepository fetches tags for a repository and processes each tag.
// Returns an error if any occurs during fetching or processing.
// If successful, it returns nil, indicating no errors were encountered.
func (c *Controller) processRepository(repo string) error {
	tags, err := c.FetchTags(repo)
	if err != nil {
		return fmt.Errorf("failed to fetch tags for repository %s: %w", repo, err)
	}

	for _, tagInfo := range tags {
		if err := c.ProcessTag(repo, tagInfo.Name, tagInfo.LastModified); err != nil {
			return fmt.Errorf("failed to process tag %s in repository %s: %w", tagInfo.Name, repo, err)
		}
	}

	return nil
}
