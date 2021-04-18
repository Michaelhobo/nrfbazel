// Package nrfbazelify converts nRF5 SDKs to use Bazel.
// This contains a Bazel BUILD file generator that reads source files
// and does the heavy lifting of resolving targets and writing BUILD files.
package nrfbazelify

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"path/filepath"
	"strings"
)

var (
  dotGraphPath = flag.String("dot_graph_path", "", "The path to write the DOT graph. Omit to skip DOT graph output.")
  dotGraphProgressionDir = flag.String("dot_graph_progression_dir", "", "The path to a directory to write the DOT graph progression. Omit to skip DOT graph progression output.")
)

// GenerateBuildFiles generates BUILD files for an nRF5 SDK.
func GenerateBuildFiles(workspaceDir, sdkDir string, verbose bool) error {
  if !filepath.IsAbs(workspaceDir) {
    return errors.New("workspace must be an absolute path")
  }
  if !filepath.IsAbs(sdkDir) {
    return errors.New("sdk_dir must be an absolute path")
  }
  if !strings.HasPrefix(sdkDir, workspaceDir) {
    return fmt.Errorf("sdk_dir is not inside workspace_dir:\nsdk_dir=%s\nworkspace_dir=%s", sdkDir, workspaceDir)
  }
  conf, err := ReadConfig(sdkDir, workspaceDir, verbose)
  if err != nil {
    return fmt.Errorf("ReadBazelifyRC: %v", err)
  }
  log.Printf("Generating BUILD files for %s", sdkDir)
  graph := NewDependencyGraph(sdkDir, workspaceDir, *dotGraphProgressionDir)
  if *dotGraphPath != "" {
    defer func(path string) {
      log.Printf("Saving dependency graph to %s", path)
      if err := graph.OutputDOTGraph(path); err != nil {
        log.Printf("OutputDOTGraph(%q): %v", path, err)
      }
    }(*dotGraphPath)
  }
  walker, err := NewSDKWalker(conf, graph)
  if err != nil {
    return fmt.Errorf("NewSDKWalker: %v", err)
  }
  unresolvedDeps, err := walker.PopulateGraph()
  if err != nil {
    return fmt.Errorf("SDKWalker.PopulateGraph: %v", err)
  }
  if len(unresolvedDeps) > 0 {
    return WriteNewHint(conf, unresolvedDeps)
  }
  if err := OutputBuildFiles(conf, graph); err != nil {
    return fmt.Errorf("OutputBuildFiles: %v", err)
  }
  if err := RemoveStaleHint(sdkDir); err != nil {
    return fmt.Errorf("removeStaleHintFile: %v", err)
  }
  return nil
}
