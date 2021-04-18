package nrfbazelify

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/Michaelhobo/nrfbazel/internal/bazel"
	"github.com/Michaelhobo/nrfbazel/internal/buildfile"
)

const (
  // We write the contents of our remap features to this file.
  bzlFilename = "remap.bzl"
)

func OutputBuildFiles(conf *Config, depGraph *DependencyGraph) error {
  files := make(map[string]*buildfile.File)

  // Convert depGraph nodes into BUILD files.
  nodes := depGraph.Nodes()
  for _, node := range nodes {
    c, err := extractBuildContents(node, depGraph)
    if err != nil {
      return err
    }
    if c == nil {
      continue
    }
    if files[c.label.Dir()] == nil {
      files[c.label.Dir()] = buildfile.New(filepath.Join(conf.WorkspaceDir, c.label.Dir()))
    }
    file := files[c.label.Dir()]
    if c.library != nil {
      file.AddLibrary(c.library)
    }
    if c.labelSetting != nil {
      file.AddLabelSetting(c.labelSetting)
    }
    if c.load != nil {
      file.AddLoad(c.load)
    }
  }

  // Make sure we load cc_library in each BUILD file.
  for _, file := range files {
    file.AddLoad(&buildfile.Load{
      Source: "@rules_cc//cc:defs.bzl",
      Symbols: []string{"cc_library"},
    })
  }

  // Write BUILD file contents.
  for _, file := range files {
    if err := file.Write(); err != nil {
      return err
    }
  }

  if conf.Remaps != nil {
    // Write remaps .bzl contents.
    remapBzlPath := filepath.Join(conf.SDKDir, bzlFilename)
    if err := os.WriteFile(remapBzlPath, conf.Remaps.BzlContents(), 0644); err != nil {
      return fmt.Errorf("WriteFile(%q): %v", remapBzlPath, err)
    }
  }

  return nil
}

type buildContents struct {
  label *bazel.Label
  library *buildfile.Library
  labelSetting *buildfile.LabelSetting
  load *buildfile.Load
}

func extractBuildContents(node Node, depGraph *DependencyGraph) (*buildContents, error) {
  switch n := node.(type) {
  case *LibraryNode:
    return libraryContents(n, depGraph), nil
  case *GroupNode:
    return groupContents(n, depGraph), nil
  case *RemapNode:
    return remapContents(n, depGraph), nil
  case *OverrideNode:
    // Override nodes are ignored, they just represent a label,
    // and don't need any rules written.
    return nil, nil
  default:
    return nil, fmt.Errorf("unknown node type for node %q", n.Label())
  }
}

func libraryContents(node *LibraryNode, depGraph *DependencyGraph) *buildContents {
  var deps []string
  depNodes := depGraph.Dependencies(node.Label())
  for _, d := range depNodes {
    deps = append(deps, d.Label().RelativeTo(node.Label()))
  }

  // Sort the srcs, hdrs, and deps so output has a deterministic order.
  // This is especially useful for tests.
  sort.Strings(node.Srcs)
  sort.Strings(node.Hdrs)
  sort.Strings(deps)

  return &buildContents{
    label: node.Label(),
    library: &buildfile.Library{
      Name: node.Label().Name(),
      Srcs: node.Srcs,
      Hdrs: node.Hdrs,
      Deps: deps,
      Includes: node.Includes,
    },
  }
}

func groupContents(node *GroupNode, depGraph *DependencyGraph) *buildContents {
  var deps []string
  depNodes := depGraph.Dependencies(node.Label())
  for _, d := range depNodes {
    deps = append(deps, d.Label().RelativeTo(node.Label()))
  }

  // Sort the srcs, hdrs, and deps so output has a deterministic order.
  // This is especially useful for tests.
  sort.Strings(node.Srcs)
  sort.Strings(node.Hdrs)
  sort.Strings(deps)
  
  return &buildContents{
    label: node.Label(),
    library: &buildfile.Library{
      Name: node.Label().Name(),
      Srcs: node.Srcs,
      Hdrs: node.Hdrs,
      Deps: deps,
    },
  }
}

func remapContents(node *RemapNode, depGraph *DependencyGraph) *buildContents {
  return &buildContents{
    label: node.Label(),
    labelSetting: node.LabelSetting,
  }
}