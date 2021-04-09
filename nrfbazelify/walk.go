package nrfbazelify

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/Michaelhobo/nrfbazel/internal/bazel"
)

func NewSDKWalker(sdkDir, workspaceDir string, graph *DependencyGraph, excludes, ignoreHeaders, includeDirs []string, includeOverrides map[string]string) (*SDKWalker, error) {
  ignoreHeadersMap := make(map[string]bool)
  for _, ignore := range ignoreHeaders {
    ignoreHeadersMap[ignore] = true
  }
  overrides := make(map[string]*bazel.Label)
  for name, override := range includeOverrides {
    label, err := bazel.ParseLabel(override)
    if err != nil {
      return nil, err
    }
    overrides[name] = label
  }

  absIncludeDirs := make([]string, 0, len(includeDirs))
  // Make all include dir paths absolute.
  for _, dir := range includeDirs {
    joined := filepath.Join(sdkDir, dir)
    abs, err := filepath.Abs(joined)
    if err != nil {
      return nil, fmt.Errorf("filepath.Abs(%q): %v", joined, err)
    }
    absIncludeDirs = append(absIncludeDirs, abs)
  }

  return &SDKWalker{
    sdkDir: sdkDir,
    workspaceDir: workspaceDir,
    graph: graph,
    excludes: excludes,
    ignoreHeaders: ignoreHeadersMap,
    includeDirs: absIncludeDirs,
    includeOverrides: overrides,
  }, nil
}

type SDKWalker struct {
  sdkDir, workspaceDir string
  graph *DependencyGraph
  excludes []string
  ignoreHeaders map[string]bool
  includeDirs []string
  includeOverrides map[string]*bazel.Label // file name -> override label
}

func (s *SDKWalker) PopulateGraph() ([]*unresolvedDep, error) {
  // Add nodes to graph and add dependencies to resolvedDeps/unresolvedDeps
  if err := filepath.Walk(s.sdkDir, s.addFilesAsNodes); err != nil {
    return nil, err
  }
  if err := s.addOverrideNodes(); err != nil {
    return nil, err
  }
  return s.addDepsAsEdges()
}

func (s *SDKWalker) addFilesAsNodes(path string, info os.FileInfo, err error) error {
  if err != nil {
    return fmt.Errorf("%s: %v", path, err)
  }
  relPath, err := filepath.Rel(s.sdkDir, path)
  if err != nil {
    return err
  }

  // Check to see if path is excluded.
  for _, exclude := range s.excludes {
    matched, err := filepath.Match(exclude, relPath)
    if err != nil {
      return err
    }
    if matched && info.IsDir() {
      return filepath.SkipDir
    }
    if matched {
      return nil
    }
  }

  // We don't care about directories
  if info.IsDir() {
    return nil
  }

  // Remove all BUILD files
  if info.Name() == "BUILD" {
    if err := os.Remove(path); err != nil {
      return fmt.Errorf("os.Remove(%s): %v", path, err)
    }
  }

  // We only want to deal with .h files
  if filepath.Ext(path) != ".h" {
    return nil
  }

  // Create Label
  dir := filepath.Dir(path)
  name := strings.TrimSuffix(info.Name(), ".h")
  label, err := bazel.NewLabel(dir, name, s.workspaceDir)
  if err != nil {
    return fmt.Errorf("bazel.NewLabel(%q, %q, %q): %v", dir, name, s.workspaceDir, err)
  }

  hdrs := []string{info.Name()}
  var srcs []string
  srcFileName := fmt.Sprintf("%s.c", name)
  if _, err := os.Stat(filepath.Join(dir, srcFileName)); err == nil {
    srcs = append(srcs, srcFileName)
  }

  if err := s.graph.AddLibraryNode(label, srcs, hdrs); err != nil {
    return fmt.Errorf("graph.AddLibraryNode(%q, %v, %v): %v", label, srcs, hdrs, err)
  }
  return nil
}

func (s *SDKWalker) addOverrideNodes() error {
  for name, label := range s.includeOverrides {
    if err := s.graph.AddOverrideNode(name, label); err != nil {
      return err
    }
  }
  return nil
}

type unresolvedDep struct {
  includedBy []*bazel.Label
  dstFileName string
  possible []*bazel.Label
}

type resolvedDep struct {
  src, dst *bazel.Label
}

func (s *SDKWalker) addDepsAsEdges() ([]*unresolvedDep, error) {
  allUnresolved := make(map[string]*unresolvedDep) // maps dstFileName -> unresolvedDep
  var allResolved []*resolvedDep

  // Look through all nodes and add each node's deps as dependencies.
  // Some dependencies can't be resolved, so we collect those to report it as an error.
  // We can't add edges into the graph until we've finished looking through all nodes,
  // in case we mess with the graph. So, we collect all the resolved deps and add them
  // at the end.
  for _, n := range s.graph.Nodes() {
    node, ok := n.(*LibraryNode)
    if !ok {
      // Skip non-Library nodes, because all other node types are resolved differently.
      continue
    }
    resolved, unresolved, err := s.readDepsOnce(node)
    if err != nil {
      return nil, fmt.Errorf("readDepsOnce: %v", err)
    }
    allResolved = append(allResolved, resolved...)

    // Deconflict all our unresolved deps using our allUnresolved map.
    for _, dep := range unresolved {
      if unresolvedDeps := allUnresolved[dep.dstFileName]; unresolvedDeps == nil {
        allUnresolved[dep.dstFileName] = dep
      } else {
        unresolvedDeps.includedBy = append(unresolvedDeps.includedBy, dep.includedBy...)
      }
    }
  }

  // Add all resolved dependencies to the graph.
  for _, dep := range allResolved {
    if err := s.graph.AddDependency(dep.src, dep.dst); err != nil {
      return nil, err
    }
  }

  // Convert unresolvedDep back into a slice.
  var out []*unresolvedDep
  for _, u := range allUnresolved {
    out = append(out, u)
  }
  return out, nil
}

func (s *SDKWalker) readDepsOnce(node *LibraryNode) ([]*resolvedDep, []*unresolvedDep, error) {
  files := make(map[string]bool)
  for _, src := range node.Srcs {
    files[src] = true
  }
  for _, hdr := range node.Hdrs {
    files[hdr] = true
  }

  // Read includes for srcs and hdrs
  deps := make(map[string]bool)
  for file := range files {
    filePath := filepath.Join(s.workspaceDir, node.Label().Dir(), file)
    includes, err := readIncludes(filePath)
    if err != nil {
      return nil, nil, fmt.Errorf("readIncludes(%q): %v", s.prettySDKPath(filePath), err)
    }
    for _, include := range includes {
      deps[include] = true
    }
  }

  // Filter the deps
  for dep := range deps {
    if s.ignoreHeaders[dep] {
      delete(deps, dep)
    } else if files[dep] {
      delete(deps, dep)
    }
  }
  
  var resolved []*resolvedDep
  var unresolved []*unresolvedDep

  // Perform a search for the file through the include_dirs in bazelifyrc,
  // and the current library's directory.
  searchPaths := make([]string, 0, len(s.includeDirs) + 1)
  searchPaths = append(searchPaths, filepath.Join(s.workspaceDir, node.Label().Dir()))
  for dep := range deps {
    // Stat all instances of the include. If we find a relative include that matches,
    // format the target and resolve it.
    for _, searchPath := range searchPaths {
      search := filepath.Clean(filepath.Join(searchPath, dep))
      info, err := os.Stat(search)
      if err != nil {
        continue
      }
      if info.IsDir() {
        continue
      }
      depLabel, err := bazel.NewLabel(searchPath, strings.TrimSuffix(dep, ".h"), s.workspaceDir)
      if err != nil {
        return nil, nil, fmt.Errorf("bazel.NewLabel(%q, %q, %q): %v", searchPath, strings.TrimSuffix(dep, ".h"), s.workspaceDir, err)
      }
      // Make sure the node is part of the graph.
      if depNode := s.graph.Node(depLabel); depNode == nil {
        continue
      }
      resolved = append(resolved, &resolvedDep{
        src: node.Label(),
        dst: depLabel,
      })
      delete(deps, dep)
      break
    }
  }

  // Look through remaining deps and see if we can find nodes that contain the file.
  for dep := range deps {
    nodes := s.graph.NodesWithFile(dep)
    if len(nodes) != 1 {
      var possible []*bazel.Label
      for _, n := range nodes {
        possible = append(possible, n.Label())
      }
      unresolved = append(unresolved, &unresolvedDep{
        includedBy: []*bazel.Label{node.Label()},
        dstFileName: dep,
        possible: possible,
      })
    } else {
      resolved = append(resolved, &resolvedDep{
        src: node.Label(),
        dst: nodes[0].Label(),
      })
    }
  }

  return resolved, unresolved, nil
}

func readIncludes(path string) ([]string, error) {
  file, err := os.Open(path)
  if err != nil {
    return nil, err
  }
  defer file.Close()

  scanner := bufio.NewScanner(file)
  var out []string
  for scanner.Scan() {
    line := scanner.Text()
    matches := includeMatcher.FindStringSubmatch(line)
    if len(matches) != 2 {
      if matches != nil {
        log.Printf("Reading includes from %s: len(%v) != 2", path, matches)
      }
      continue
    }
    out = append(out, matches[1])
  }
  return out, nil
}

func (s *SDKWalker) prettySDKPath(path string) string {
  if !strings.HasPrefix(path, s.sdkDir) {
    return fmt.Sprintf("<WARNING: not in SDK %q>", s.sdkDir)
  }
  return "<SDK>" + strings.TrimPrefix(path, s.sdkDir)
}