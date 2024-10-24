package upload

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/flacatus/oras-puller/pkg/controller/oci"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/spf13/cobra"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/file"
	"oras.land/oras-go/v2/content/memory"
	"oras.land/oras-go/v2/errdef"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/credentials"
	"oras.land/oras-go/v2/registry/remote/retry"
)

// Pre-defined annotation keys for annotation file
const (
	AnnotationManifest = "$manifest"
	AnnotationConfig   = "$config"
)

// uploadOptions holds the configuration for the upload command
type uploadOptions struct {
	dest         string
	artifactType string
}

// Initialize a global instance of uploadOptions
var opts = &uploadOptions{}

// uploadCmd represents the upload command
var uploadCmd = &cobra.Command{
	Use:   "upload",
	Short: "Upload files and folders to OCI storage",
	Long: `Upload files to an OCI (Open Container Initiative) compliant repository.

Examples:
  - Upload multiple files:
      konflux-oci-artifacts upload --dest oci://myrepo file1.tar file2.tar

  - Upload multiple folders:
      konflux-oci-artifacts upload --dest oci://myrepo ./folder1 ./folder2

  - Upload both files and folders:
      konflux-oci-artifacts upload --dest oci://myrepo file1.tar ./folder1`,
	RunE: func(cmd *cobra.Command, args []string) error {
		pula := args[1:]
		fmt.Println("###")
		fmt.Println(pula)
		fmt.Println("####a")
		fmt.Println(opts.dest)
		// Ensure the 'dest' flag is provided
		if opts.dest == "" {
			return fmt.Errorf("destination must be specified using --dest flag")
		}

		store, err := file.New("")
		if err != nil {
			return err
		}
		defer store.Close()
		memoryStore := memory.New()
		ociController, err := oci.NewController("./test", "./test-cache")

		repoz, tagz, err := parseRepoAndTag("quay.io/konflux-test-storage/konflux-team/e2e-tests:konflux-e2e-fdv6k")
		if err != nil {
			return err
		}

		// Call ProcessTag to get details of the tag (implement as needed)
		if err := ociController.ProcessTag(repoz, tagz, time.Now().Format(time.RFC1123)); err != nil {
			return fmt.Errorf("failed to fetch tag: %v", err)
		}

		// Find the deepest directory containing files
		deepestDir, _ := findDirWithFiles("./test")
		fmt.Println(deepestDir)
		pula = append(pula, deepestDir)

		if deepestDir != "" {
			fmt.Printf("Deepest directory with files: %s\n", deepestDir)
		} else {
			fmt.Println("No directories with files found.")
		}

		ann, _ := ociController.FetchOCIContainerAnnotations(repoz, tagz)

		fmt.Println(ann.Annotations)

		packOpts := oras.PackManifestOptions{
			ManifestAnnotations: ann.Annotations,
		}

		descs, err := loadFiles(context.Background(), store, make(map[string]map[string]string), pula)

		if err != nil {
			return err
		}

		packOpts.Layers = descs

		root, err := oras.PackManifest(context.Background(), memoryStore, 2, "application/vnd.unknown.artifact.v1", packOpts)
		if err != nil {
			return err
		}
		if err = memoryStore.Tag(context.Background(), root, root.Digest.String()); err != nil {
			return err
		}

		union := MultiReadOnlyTarget(memoryStore, store)
		fmt.Println(union)
		// Simulate upload logic
		fmt.Printf("Successfully uploaded artifacts to %s with artifact type %s", opts.dest, opts.artifactType)

		reg := "quay.io"
		repo, err := remote.NewRepository(reg + "/konflux-test-storage/konflux-team/e2e-tests")
		if err != nil {
			panic(err)
		}

		credStore, err := credentials.NewStoreFromDocker(credentials.StoreOptions{})
		if err != nil {
			fmt.Errorf("failed to create credential store: %w", err)
		}
		repo.Client = &auth.Client{
			Client:     retry.DefaultClient,
			Cache:      auth.NewCache(),
			Credential: credentials.Credential(credStore),
		}
		_, err = oras.Copy(context.Background(), union, root.Digest.String(), repo, "konflux-e2e-fdv6k", oras.DefaultCopyOptions)
		if err != nil {
			fmt.Println(err)
		}

		return nil
	},
}

// Init initializes the upload command and its flags
func Init() *cobra.Command {
	// Bind flags to the global opts instance
	uploadCmd.Flags().StringVarP(&opts.dest, "dest", "D", "", "Destination repository (e.g., oci://myrepo)")
	uploadCmd.Flags().StringVarP(&opts.artifactType, "artifact-type", "T", "", "Set the artifact type for the upload")

	// Mark destination as a required flag
	uploadCmd.MarkFlagRequired("dest")

	return uploadCmd
}

type multiReadOnlyTarget struct {
	targets []oras.ReadOnlyTarget
}

// MultiReadOnlyTarget returns a ReadOnlyTarget that combines multiple targets.
func MultiReadOnlyTarget(targets ...oras.ReadOnlyTarget) oras.ReadOnlyTarget {
	return &multiReadOnlyTarget{
		targets: targets,
	}
}

// Exists returns true if the content exists in any of the targets.
// multiReadOnlyTarget does not implement Exists() because it's read-only.
func (m *multiReadOnlyTarget) Exists(ctx context.Context, target ocispec.Descriptor) (bool, error) {
	return false, errors.New("MultiReadOnlyTarget.Exists() is not implemented")
}

// Resolve resolves the reference to a descriptor from the targets in order and
// return first found descriptor. If no descriptor is found, it returns
// ErrNotFound.
func (m *multiReadOnlyTarget) Resolve(ctx context.Context, ref string) (ocispec.Descriptor, error) {
	lastErr := errdef.ErrNotFound
	for _, c := range m.targets {
		desc, err := c.Resolve(ctx, ref)
		if err == nil {
			return desc, nil
		}
		if !errors.Is(err, errdef.ErrNotFound) {
			return ocispec.Descriptor{}, err
		}
		lastErr = err
	}
	return ocispec.Descriptor{}, lastErr
}

// Fetch fetches the content from the targets in order and return first found
// content. If no content is found, it returns ErrNotFound.
func (m *multiReadOnlyTarget) Fetch(ctx context.Context, target ocispec.Descriptor) (io.ReadCloser, error) {
	lastErr := errdef.ErrNotFound
	for _, c := range m.targets {
		rc, err := c.Fetch(ctx, target)
		if err == nil {
			return rc, nil
		}
		if !errors.Is(err, errdef.ErrNotFound) {
			return nil, err
		}
		lastErr = err
	}
	return nil, lastErr
}

// parseRepoAndTag extracts the repository and tag from the given repo flag.
func parseRepoAndTag(repoFlag string) (string, string, error) {
	// Ensure the repoFlag starts with 'quay.io/'
	if !strings.HasPrefix(repoFlag, "quay.io/") {
		return "", "", fmt.Errorf("the repository must start with 'quay.io/'")
	}

	// Remove 'quay.io/' prefix and split the repo and tag using the ':' character
	repoFlag = strings.TrimPrefix(repoFlag, "quay.io/")
	parts := strings.SplitN(repoFlag, ":", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("tag is missing in the repo flag")
	}

	return parts[0], parts[1], nil
}

// Function to check if a directory contains files
func containsFiles(path string) bool {
	files, err := os.ReadDir(path)
	if err != nil {
		return false
	}
	for _, file := range files {
		if !file.IsDir() {
			// Return true if any file is found in the directory
			return true
		}
	}
	// Return false if no files are found
	return false
}

// Function to recursively find the first directory with files and stop further traversal
func findDirWithFiles(root string) (string, error) {
	var dirWithFiles string

	// Walk function to process each directory and file
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// Only check directories
		if info.IsDir() {
			// Check if the directory contains files
			if containsFiles(path) {
				// If it contains files, set this as the directory and stop further recursion
				dirWithFiles = path
				return filepath.SkipDir
			}
		}
		return nil
	})

	// Return the directory and any errors encountered
	return dirWithFiles, err
}
