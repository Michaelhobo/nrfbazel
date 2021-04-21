// Package nrfbazelify converts nRF5 SDKs to use Bazel.
// This contains a Bazel BUILD file generator that reads source files
// and does the heavy lifting of resolving targets and writing BUILD files.
package nrfbazelify

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

var (
  fullGraph = flag.Bool("full_graph", false, "Whether to create a DOT graph of the full graph.")
  progressionGraphs = flag.Bool("progression_graphs", false, "Whether to create a DOT graph for each change in the graph.")
  namedGroupGraphs = flag.Bool("named_group_graphs", false, "Whether to create a DOT graph for each named group.")
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

  // Setup .bazelify-out directory.
  bazelifyOutDOTDir := filepath.Join(sdkDir, ".bazelify-out", "dot")

  fullGraphDir := filepath.Join(bazelifyOutDOTDir, "full_graph")
  progressionGraphsDir := filepath.Join(bazelifyOutDOTDir, "progression_graphs")
  namedGroupGraphsDir := filepath.Join(bazelifyOutDOTDir, "named_group_graphs")

  // Remove all outputs from .bazelify-out file.
  for _, dir := range []string{fullGraphDir, progressionGraphsDir, namedGroupGraphsDir} {
    if err := os.RemoveAll(dir); err != nil {
      return fmt.Errorf("os.RemoveAll(%q): %v", dir, err)
    }
  }

  // Set up progression graph.
  var progGraphDir string
  if *progressionGraphs {
    if err := os.MkdirAll(progressionGraphsDir, 0755); err != nil {
      return fmt.Errorf("MkdirAll(%q): %v", progressionGraphsDir, err)
    }
    progGraphDir = progressionGraphsDir
  }

  graph := NewDependencyGraph(conf, progGraphDir)

  // Set up output of the full DOT graph.
  if *fullGraph {
    if err := os.MkdirAll(fullGraphDir, 0755); err != nil {
      return fmt.Errorf("MkdirAll(%q): %v", fullGraphDir, err)
    }
    defer func() {
      log.Printf("Saving dependency graph to %s", fullGraphDir)
      if err := graph.OutputDOTGraph(filepath.Join(fullGraphDir, "full_graph.dot")); err != nil {
        log.Printf("OutputDOTGraph(%q): %v", fullGraphDir, err)
      }
    }()
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
    return WriteUnresolvedDepsHint(conf, unresolvedDeps)
  }

  unnamedGroups, err := NameGroups(conf, graph)
  if err != nil {
    return fmt.Errorf("NameGroups: %v", err)
  }
  if len(unnamedGroups) > 0 {
    return WriteUnnamedGroupsHint(conf, unnamedGroups)
  }

  if err := OutputBuildFiles(conf, graph); err != nil {
    return fmt.Errorf("OutputBuildFiles: %v", err)
  }

  if err := RemoveStaleHint(sdkDir); err != nil {
    return fmt.Errorf("removeStaleHintFile: %v", err)
  }

  stats, err := NewGraphStats(conf, graph)
  if err != nil {
    return fmt.Errorf("NewGraphStats: %v", err)
  }
  log.Print(stats.GenerateReport())

  // Now that the graph is complete, write out all named groups for visualization.
  if *namedGroupGraphs {
    if err := os.MkdirAll(namedGroupGraphsDir, 0755); err != nil {
      return fmt.Errorf("MkdirAll(%q): %v", namedGroupGraphsDir, err)
    }
    if err := stats.WriteNamedGroupGraphs(namedGroupGraphsDir); err != nil {
      return fmt.Errorf("WriteNamedGroupGraphs: %v", err)
    }
  }

  return nil
}
