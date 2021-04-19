package nrfbazelify

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Michaelhobo/nrfbazel/internal/bazel"
)

var (
  includeMatcher = regexp.MustCompile("^\\s*#include\\s+\"(.+)\".*$")
)

func NewSDKWalker(conf *Config, graph *DependencyGraph) (*SDKWalker, error) {
  return &SDKWalker{
    conf: conf,
    graph: graph,
  }, nil
}

type SDKWalker struct {
  conf *Config
  graph *DependencyGraph
}

func (s *SDKWalker) PopulateGraph() ([]*unresolvedDep, error) {
  if err := s.addSourceSetFiles(); err != nil {
    return nil, fmt.Errorf("addSourceSetFiles: %v", err)
  }
  // Add nodes to graph and add dependencies to resolvedDeps/unresolvedDeps
  if err := filepath.Walk(s.conf.SDKDir, s.addFilesAsNodes); err != nil {
    return nil, fmt.Errorf("filepath.Walk: %v", err)
  }
  if err := s.addOverrideNodes(); err != nil {
    return nil, fmt.Errorf("addOverrideNodes: %v", err)
  }
  if err := s.addRemapNodes(); err != nil {
    return nil, fmt.Errorf("addRemapNodes: %v", err)
  }
  unresolved, err := s.addDepsAsEdges()
  if err != nil {
    return nil, fmt.Errorf("addDepsAsEdges: %v", err)
  }
  return unresolved, nil
}

func (s *SDKWalker) addSourceSetFiles() error {
  for labelStr, files := range s.conf.SourceSets {
    label, err := bazel.ParseLabel(labelStr)
    if err != nil {
      return fmt.Errorf("bazel.ParseLabel(%q): %v", labelStr, err)
    }
    if err := s.graph.AddLibraryNode(label, files.Srcs, files.Hdrs, []string{"."}); err != nil {
      return fmt.Errorf("AddLibraryNode(%q): %v", label, err)
    }
  }
  return nil
}

func (s *SDKWalker) addFilesAsNodes(path string, info os.FileInfo, err error) error {
  if err != nil {
    return fmt.Errorf("%s: %v", path, err)
  }
  // Check to see if path is excluded.
  for _, exclude := range s.conf.Excludes {
    matched, err := filepath.Match(exclude, path)
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

  // Source set files have already been added, so skip them here.
  if s.conf.SourceSetsByFile[path] != nil {
    return nil
  }

  // Create Label
  dir := filepath.Dir(path)
  name := strings.TrimSuffix(info.Name(), ".h")
  label, err := bazel.NewLabel(dir, name, s.conf.WorkspaceDir)
  if err != nil {
    return fmt.Errorf("bazel.NewLabel(%q, %q, %q): %v", dir, name, s.conf.WorkspaceDir, err)
  }

  hdrLabel, err := bazel.NewLabel(dir, info.Name(), s.conf.WorkspaceDir)
  if err != nil {
    return fmt.Errorf("bazel.NewLabel(%q, %q): %v", dir, info.Name(), err)
  }
  hdrs := []*bazel.Label{hdrLabel}
  var srcs []*bazel.Label
  srcFileName := fmt.Sprintf("%s.c", name)
  if _, err := os.Stat(filepath.Join(dir, srcFileName)); err == nil {
    srcLabel, err := bazel.NewLabel(dir, srcFileName, s.conf.WorkspaceDir)
    if err != nil {
      return fmt.Errorf("bazel.NewLabel(%q, %q): %v", dir, srcFileName, err)
    }
    srcs = append(srcs, srcLabel)
  }

  if err := s.graph.AddLibraryNode(label, srcs, hdrs, []string{"."}); err != nil {
    return fmt.Errorf("graph.AddLibraryNode(%q, %v, %v): %v", label, srcs, hdrs, err)
  }
  return nil
}

func (s *SDKWalker) addOverrideNodes() error {
  for name, label := range s.conf.IncludeOverrides {
    if err := s.graph.AddOverrideNode(name, label); err != nil {
      return err
    }
  }
  return nil
}

func (s *SDKWalker) addRemapNodes() error {
  if s.conf.Remaps == nil {
    return nil
  }
  for fileName, labelSetting := range s.conf.Remaps.LabelSettings() {
    label, err := bazel.NewLabel(s.conf.SDKDir, labelSetting.Name, s.conf.WorkspaceDir)
    if err != nil {
      return fmt.Errorf("bazel.NewLabel(%q): %v", labelSetting.Name, err)
    }
    if err := s.graph.AddRemapNode(label, fileName, labelSetting); err != nil {
      return fmt.Errorf("AddRemapNode(%q): %v", label, err)
    }
  }
  for _, lib := range s.conf.Remaps.Libraries() {
    label, err := bazel.NewLabel(s.conf.SDKDir, lib.Name, s.conf.WorkspaceDir)
    if err != nil {
      return fmt.Errorf("bazel.NewLabel(%q): %v", lib.Name, err)
    }
    dir := filepath.Join(s.conf.WorkspaceDir, label.Dir())
    var srcs, hdrs []*bazel.Label
    for _, src := range lib.Srcs {
      srcLabel, err := bazel.NewLabel(dir, src, s.conf.WorkspaceDir)
      if err != nil {
        return fmt.Errorf("bazel.NewLabel(%q, %q): %v", dir, src, err)
      }
      srcs = append(srcs, srcLabel)
    }
    for _, hdr := range lib.Hdrs {
      hdrLabel, err := bazel.NewLabel(dir, hdr, s.conf.WorkspaceDir)
      if err != nil {
        return fmt.Errorf("bazel.NewLabel(%q, %q): %v", dir, hdr, err)
      }
      hdrs = append(hdrs, hdrLabel)
    }
    if err := s.graph.AddLibraryNode(label, srcs, hdrs, lib.Includes); err != nil {
      return fmt.Errorf("AddLibraryNode(%q): %v", label, err)
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
  srcsHdrs := make(map[string]*bazel.Label)
  for _, src := range node.Srcs {
    srcsHdrs[src.String()] = src
  }
  for _, hdr := range node.Hdrs {
    srcsHdrs[hdr.String()] = hdr
  }

  // Read includes for srcs and hdrs
  deps := make(map[string]bool)
  for _, fileLabel := range srcsHdrs {
    filePath := filepath.Join(s.conf.WorkspaceDir, fileLabel.Dir(), fileLabel.Name())
    includes, err := readIncludes(filePath)
    if err != nil {
      return nil, nil, fmt.Errorf("readIncludes(%q): %v", s.prettySDKPath(filePath), err)
    }
    for _, include := range includes {
      deps[include] = true
    }
  }

  // Filter the deps that should be ignored.
  for dep := range deps {
    if s.conf.IgnoreHeaders[dep] {
      delete(deps, dep)
    }
  }

  // Filter the deps that match up with files in the srcs/hdrs of this node.
  for dep := range deps {
    dir := filepath.Join(s.conf.WorkspaceDir, node.Label().Dir())
    depLabel, err := bazel.NewLabel(dir, dep, s.conf.WorkspaceDir)
    if err != nil {
      return nil, nil, fmt.Errorf("bazel.NewLabel(%q, %q): %v", dir, dep, err)
    }
    if srcsHdrs[depLabel.String()] != nil {
      delete(deps, dep)
    }
  }
  
  var resolved []*resolvedDep
  var unresolved []*unresolvedDep

  // If the node is overridden, use the override.
  for dep := range deps {
    if !s.graph.IsFileOverridden(dep) {
      continue
    }
    resolved = append(resolved, &resolvedDep{
      src: node.Label(),
      // If the file is overridden, we're guaranteed to have exactly 1 returned Node.
      dst: s.graph.NodesWithFile(dep)[0].Label(),
    })
    delete(deps, dep)
  }

  // Perform a search for the file through the include_dirs in bazelifyrc,
  // and the current library's directory.
  searchPaths := make([]string, 0, len(s.conf.IncludeDirs) + 1)
  searchPaths = append(searchPaths, filepath.Join(s.conf.WorkspaceDir, node.Label().Dir()))
  searchPaths = append(searchPaths, s.conf.IncludeDirs...)
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
      depLabel, err := bazel.NewLabel(filepath.Dir(search), strings.TrimSuffix(filepath.Base(search), ".h"), s.conf.WorkspaceDir)
      if err != nil {
        return nil, nil, fmt.Errorf("bazel.NewLabel(%q, %q, %q): %v", searchPath, strings.TrimSuffix(dep, ".h"), s.conf.WorkspaceDir, err)
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
  if !strings.HasPrefix(path, s.conf.SDKDir) {
    return fmt.Sprintf("<WARNING: not in SDK %q>", s.conf.SDKDir)
  }
  return "<SDK>" + strings.TrimPrefix(path, s.conf.SDKDir)
}