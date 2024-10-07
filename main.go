package main

import (
	"log"

	"github.com/flacatus/oras-puller/pkg/controller/oci"
)

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	// Initialize the controller
	controller := oci.NewController("/home/flacatus/WORKSPACE/qe/oras-sdk/out", "/home/flacatus/WORKSPACE/qe/oras-sdk/blobs")

	repositories := []string{
		"konflux-test-storage/konflux-team/integration-service",
		"konflux-test-storage/konflux-team/release-service",
		"konflux-test-storage/konflux-team/e2e-tests",
		"konflux-test-storage/konflux-team/image-controller",
		"konflux-test-storage/konflux-team/build-service",
		"konflux-test-storage/rhtap-team/rhtap-e2e",
	}

	var allErrors []error
	for _, repo := range repositories {
		log.Println("Processing repository:", repo)

		errors := controller.ProcessRepositories([]string{repo})
		allErrors = append(allErrors, errors...)
	}

	if len(allErrors) > 0 {
		log.Println("Errors encountered during processing:")
		for _, err := range allErrors {
			log.Printf(" - %v\n", err)
		}
	} else {
		log.Println("No errors encountered during processing.")
	}
}
