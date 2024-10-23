package upload

import (
	"context"
	"errors"
	"fmt"
	"io"

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

		packOpts := oras.PackManifestOptions{
			ManifestAnnotations: map[string]string{"$config": ""},
		}
		fmt.Println(packOpts)
		store, err := file.New("")
		if err != nil {
			return err
		}
		defer store.Close()
		memoryStore := memory.New()

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
		_, err = oras.Copy(context.Background(), union, root.Digest.String(), repo, "konflux-e2e-vtmbk", oras.DefaultCopyOptions)
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
