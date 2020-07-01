package main

import (
	"flag"
	"log"

	"github.com/Michaelhobo/nrfbazel/nrfbazelify"
)

var (
	workspaceDir = flag.String("workspace", "", "The Bazel WORKSPACE directory. Absolute path required.")
	sdkDir       = flag.String("sdk", "", "The path to the nrf52 SDK's root directory. Absolute path required.")
)

func main() {
	flag.Parse()
	log.Print("Generating BUILD files for %s", *sdkDir)
	if err := nrfbazelify.GenerateBuildFiles(*workspaceDir, *sdkDir); err != nil {
		log.Fatalf("Failed to generate BUILD files: %v", err)
	}
	log.Printf("Successfully generated BUILD files for %s", *sdkDir)
}
