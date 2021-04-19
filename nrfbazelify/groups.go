package nrfbazelify

import (
  "fmt"
  "path/filepath"
  "sort"

  "github.com/Michaelhobo/nrfbazel/internal/bazel"
)

// NameGroups sets the name of all GroupNodes in the graph, and returns any nodes that haven't been named.
func NameGroups(conf *Config, depGraph *DependencyGraph) ([]*GroupNode, error) {
  var out []*GroupNode
  for _, node := range depGraph.Nodes() {
    groupNode, isGroupNode := node.(*GroupNode)
    if !isGroupNode {
      continue
    }
    if len(groupNode.Hdrs) < 2 {
      return nil, fmt.Errorf("len(%q Hdrs)=%d, must be at least 2", groupNode.Label(), len(groupNode.Hdrs))
    }
    
    // Look up this group in the named groups by first and last header.
    var hdrs []string
    for _, hdr := range groupNode.Hdrs {
      hdrs = append(hdrs, hdr.FileRelativeTo(groupNode.Label().Dir()))
    }
    sort.Strings(hdrs)
    if conf.NamedGroups[hdrs[0]] == nil || conf.NamedGroups[hdrs[0]][hdrs[len(hdrs) - 1]] == "" {
      out = append(out, groupNode)
      continue
    }

    // Change the label for the node to reflect the new name.
    name := conf.NamedGroups[hdrs[0]][hdrs[len(hdrs) - 1]]
    dir := filepath.Join(conf.WorkspaceDir, groupNode.Label().Dir())
    newLabel, err := bazel.NewLabel(dir, name, conf.WorkspaceDir)
    if err != nil {
      return nil, fmt.Errorf("bazel.NewLabel(%q, %q): %v", dir, name, err)
    }
    depGraph.ChangeLabel(groupNode.Label(), newLabel)
  }
  return out, nil
}
