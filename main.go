package main

import (
	"fmt"
	"log"

	"github.com/flacatus/oras-puller/cmd/download"
	"github.com/flacatus/oras-puller/cmd/upload"
	"github.com/spf13/cobra"
)

func main() {
	// Create the root command
	rootCmd := &cobra.Command{
		Use:   "konflux-oci-artifacts",
		Short: "CLI to manage OCI artifacts storage",
		Long: `
Konflux OCI Artifacts is a CLI tool designed to help users manage OCI artifact storage.
It supports operations such as uploading, downloading, and listing OCI artifacts.`,
	}

	// Custom Help function for the root command
	rootCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		fmt.Println(`
This CLI helps you manage OCI artifacts storage. You can upload, download, and list OCI artifacts using the following commands:

Usage:
  konflux-oci-artifacts [command]

Available Commands:
  upload      Upload an artifact to OCI storage
  download    Download an artifact from OCI storage

Examples:
  Upload:
    konflux-oci-artifacts upload --file=myartifact.tar --dest=oci://myrepo

  Download:
    konflux-oci-artifacts download --repo=oci://myrepo:tag
    konflux-oci-artifacts download --repos oci://repo1 oci://repo2 --since 4h

Flags:
  -h, --help   help for konflux-oci-artifacts

Use "konflux-oci-artifacts [command] --help" for more information about a command.`)
	})

	// Add subcommands
	rootCmd.AddCommand(upload.Init())
	rootCmd.AddCommand(download.Init())

	// Execute the root command
	if err := rootCmd.Execute(); err != nil {
		log.Fatalf("ERROR: %v", err)
	}
}
