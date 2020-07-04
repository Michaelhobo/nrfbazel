package main

import (
	"flag"
	"log"
	"os"

	"github.com/Michaelhobo/nrfbazel/nrfbazelify"
)

var (
	workspaceDir = flag.String("workspace", "", "The Bazel WORKSPACE directory. Absolute path required.")
	sdkDir       = flag.String("sdk", "", "The path to the nrf52 SDK's root directory. Absolute path required.")
)

func init() {
	flag.Usage = func() {
		log.Print(`
nrfbazelify converts an nrf5 SDK to Bazel (https://bazel.build).

Usage: nrfbazelify --workspace=<absolute dir> --sdk=<absolute dir>

WARNING: nrfbazelify will delete all existing BUILD files in the directory
specified by --sdk

nrfbazelify reads options from the .bazelifyrc file at the root of the SDK.
You may be prompted to supply target overrides if nrfbazelify cannot resolve
all the dependencies.

Original program written by Michael Ho. For questions and issues, please
file issues at https://github.com/Michaelhobo/nrfbazel

Flags:
`)
		flag.PrintDefaults()
	}
}

func main() {
	flag.Parse()
	if *workspaceDir == "" || *sdkDir == "" {
		flag.Usage()
		os.Exit(1)
	}
	log.Printf("Generating BUILD files for %s", *sdkDir)
	if err := nrfbazelify.GenerateBuildFiles(*workspaceDir, *sdkDir); err != nil {
		log.Fatalf("Failed to generate BUILD files: %v", err)
	}
	log.Printf("Successfully generated BUILD files for %s", *sdkDir)
}
