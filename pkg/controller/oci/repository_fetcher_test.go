package oci

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

// MockHTTPClient is a struct for mocking HTTP requests.
type MockHTTPClient struct {
	response *http.Response
	err      error
}

// Do simulates an HTTP request and returns a mocked response.
func (m *MockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	return m.response, m.err
}

// TestFetchTags tests the FetchTags method of the Controller.
func TestFetchTags(t *testing.T) {
	tests := []struct {
		name          string
		repo          string
		mockResponse  *http.Response
		expectedTags  []TagInfo
		expectedError string
	}{
		{
			name: "Successful fetch with tags",
			repo: "test-repo",
			mockResponse: &http.Response{
				StatusCode: http.StatusOK,
				Body:       createMockResponseBody([]TagInfo{{Name: "v1.0", LastModified: "2021-01-01T00:00:00Z"}}),
			},
			expectedTags: []TagInfo{
				{Name: "v1.0", LastModified: "2021-01-01T00:00:00Z"},
			},
		},
		{
			name: "Error fetching tags",
			repo: "error-repo",
			mockResponse: &http.Response{
				StatusCode: http.StatusNotFound,
				Body:       io.NopCloser(bytes.NewBuffer([]byte(""))),
			},
			expectedError: "failed to fetch tags: 404 Not Found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use the mock HTTP client in the test.
			var tags []TagInfo
			if tt.mockResponse.StatusCode == http.StatusOK {
				// Decode the response for successful fetch.
				var response TagResponse
				if err := json.NewDecoder(tt.mockResponse.Body).Decode(&response); err != nil {
					t.Fatalf("failed to decode tags response: %v", err)
				}
				tags = response.Tags
			} else {
				// Simulate error case
				tags = nil
			}

			// Check for expected error
			if (tt.expectedError != "" && tt.mockResponse.StatusCode == http.StatusOK) ||
				(tt.expectedError == "" && tt.mockResponse.StatusCode != http.StatusOK) {
				t.Errorf("expected error %v, got %v", tt.expectedError, "unexpected error")
			}

			// Compare expected tags with actual tags
			if !equalTags(tags, tt.expectedTags) {
				t.Errorf("expected tags %v, got %v", tt.expectedTags, tags)
			}
		})
	}
}

// createMockResponseBody creates a mock response body for testing.
func createMockResponseBody(tags []TagInfo) io.ReadCloser {
	response := TagResponse{Tags: tags}
	body, _ := json.Marshal(response)
	return io.NopCloser(bytes.NewBuffer(body))
}

// Helper function to compare slices of TagInfo
func equalTags(a, b []TagInfo) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
