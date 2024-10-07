package oci

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// Constants for OCI API configuration
const (
	quayAPITagsURL = "https://quay.io/api/v1/repository/"
	perPageTags    = 100
)

// TagInfo represents a tag in a repository, including its name and the last modified date.
// This struct is used to store information about individual tags returned from the Quay API.
type TagInfo struct {
	// The name of the tag
	Name string `json:"name"`

	// The date and time when the tag was last modified
	LastModified string `json:"last_modified"`
}

// TagResponse is used to decode the JSON response from the Quay API that contains a list of tags.
// This struct represents the expected structure of the API response for tag retrieval.
type TagResponse struct {
	// A slice of TagInfo structs representing the tags in the response
	Tags []TagInfo `json:"tags"`
}

// FetchTags fetches tags for a repository from Quay.
// It paginates through the results, retrieving all available tags for the specified repository.
func (c *Controller) FetchTags(repo string) ([]TagInfo, error) {
	var tags []TagInfo
	page := 1

	for {
		url := c.buildTagsURL(repo, page)
		response, err := c.sendTagsRequest(url)
		if err != nil {
			return nil, err
		}

		if len(response.Tags) == 0 {
			break
		}

		tags = append(tags, response.Tags...)
		page++
	}

	return tags, nil
}

// buildTagsURL constructs the tags API URL for a specific repository and page.
// It formats the URL with the base URL, repository name, number of tags per page, and the current page number.
func (c *Controller) buildTagsURL(repo string, page int) string {
	return fmt.Sprintf("%s%s/tag/?limit=%d&page=%d", quayAPITagsURL, repo, perPageTags, page)
}

// sendTagsRequest sends a GET request to the provided URL and decodes the response into a TagResponse struct.
// It returns an error if the request fails or if the response cannot be decoded.
func (c *Controller) sendTagsRequest(url string) (*TagResponse, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch tags from URL %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch tags: %s", resp.Status)
	}

	var response TagResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode tags response: %w", err)
	}

	return &response, nil
}
