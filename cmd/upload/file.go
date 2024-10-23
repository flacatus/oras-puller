package upload

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/content/file"
)

// Parse parses file reference on unix.
func Parse(reference string, defaultMetadata string) (filePath, metadata string, err error) {
	i := strings.LastIndex(reference, ":")
	if i < 0 {
		filePath, metadata = reference, defaultMetadata
	} else {
		filePath, metadata = reference[:i], reference[i+1:]
	}
	if filePath == "" {
		return "", "", fmt.Errorf("found empty file path in %q", reference)
	}
	return filePath, metadata, nil
}

func loadFiles(ctx context.Context, store *file.Store, annotations map[string]map[string]string, fileRefs []string) ([]ocispec.Descriptor, error) {
	var files []ocispec.Descriptor
	for _, fileRef := range fileRefs {
		filename, mediaType, err := Parse(fileRef, "")
		if err != nil {
			return nil, err
		}

		// get shortest absolute path as unique name
		name := filepath.Clean(filename)
		if !filepath.IsAbs(name) {
			name = filepath.ToSlash(name)
		}

		file, err := addFile(ctx, store, name, mediaType, filename)
		if err != nil {
			return nil, err
		}
		if value, ok := annotations[filename]; ok {
			if file.Annotations == nil {
				file.Annotations = value
			} else {
				for k, v := range value {
					file.Annotations[k] = v
				}
			}
		}
		files = append(files, file)
	}
	return files, nil
}

func addFile(ctx context.Context, store *file.Store, name string, mediaType string, filename string) (ocispec.Descriptor, error) {
	file, err := store.Add(ctx, name, mediaType, filename)
	if err != nil {
		var pathErr *fs.PathError
		if errors.As(err, &pathErr) {
			err = pathErr
		}
		return ocispec.Descriptor{}, err
	}
	return file, nil
}
