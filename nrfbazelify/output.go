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
    contents, err := extractBuildContents(node, depGraph)
    if err != nil {
      return err
    }
    for _, c := range contents {
      if files[c.dir] == nil {
        files[c.dir] = buildfile.New(filepath.Join(conf.WorkspaceDir, c.dir))
      }
      file := files[c.dir]
      if c.library != nil {
        file.AddLibrary(c.library)
      }
      if c.labelSetting != nil {
        file.AddLabelSetting(c.labelSetting)
      }
      if c.load != nil {
        file.AddLoad(c.load)
      }
      for _, export := range c.exportFiles {
        file.ExportFile(export)
      }
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
  dir string // The directory of this BUILD file, relative to workspaceDir.
  library *buildfile.Library
  labelSetting *buildfile.LabelSetting
  load *buildfile.Load
  exportFiles []string
}

func extractBuildContents(node Node, depGraph *DependencyGraph) ([]*buildContents, error) {
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

func libraryContents(node *LibraryNode, depGraph *DependencyGraph) []*buildContents {
  var deps []string
  depNodes := depGraph.Dependencies(node.Label())
  for _, d := range depNodes {
    deps = append(deps, d.Label().RelativeTo(node.Label()))
  }

  // Sort the srcs, hdrs, and deps so output has a deterministic order.
  // This is especially useful for tests.
  var srcs, hdrs []string
  for _, src := range node.Srcs {
    srcs = append(srcs, src.FileRelativeTo(node.Label().Dir()))
  }
  for _, hdr := range node.Hdrs {
    hdrs = append(hdrs, hdr.FileRelativeTo(node.Label().Dir()))
  }
  sort.Strings(srcs)
  sort.Strings(hdrs)
  sort.Strings(deps)

  return []*buildContents{{
    dir: node.Label().Dir(),
    library: &buildfile.Library{
      Name: node.Label().Name(),
      Srcs: srcs,
      Hdrs: hdrs,
      Deps: deps,
      Includes: node.Includes,
    },
  }}
}

func groupContents(node *GroupNode, depGraph *DependencyGraph) []*buildContents {
  var deps []string
  depNodes := depGraph.Dependencies(node.Label())
  for _, d := range depNodes {
    deps = append(deps, d.Label().RelativeTo(node.Label()))
  }

  // Sort the srcs, hdrs, and deps so output has a deterministic order.
  // This is especially useful for tests.
  var srcs, hdrs []string
  for _, src := range node.Srcs {
    srcs = append(srcs, src.FileRelativeTo(node.Label().Dir()))
  }
  for _, hdr := range node.Hdrs {
    hdrs = append(hdrs, hdr.FileRelativeTo(node.Label().Dir()))
  }
  sort.Strings(srcs)
  sort.Strings(hdrs)
  sort.Strings(deps)
  out := []*buildContents{{
    dir: node.Label().Dir(),
    library: &buildfile.Library{
      Name: node.Label().Name(),
      Srcs: srcs,
      Hdrs: hdrs,
      Deps: deps,
    },
  }}

  // Add build contents for each file that needs exporting.
  var labels []*bazel.Label
  labels = append(labels, node.Srcs...)
  labels = append(labels, node.Hdrs...)
  exportFilesContents := make(map[string]*buildContents)
  for _, l := range labels {
    // We don't need to export files that are in the same directory.
    if l.Dir() == node.Label().Dir() {
      continue
    }
    if exportFilesContents[l.Dir()] == nil {
      exportFilesContents[l.Dir()] = &buildContents{
        dir: l.Dir(),
      }
    }
    exportFilesContents[l.Dir()].exportFiles = append(exportFilesContents[l.Dir()].exportFiles, l.Name())
  }

  for _, c := range exportFilesContents {
    out = append(out, c)
  }

  return out
}

func remapContents(node *RemapNode, depGraph *DependencyGraph) []*buildContents {
  return []*buildContents{{
    dir: node.Label().Dir(),
    labelSetting: node.LabelSetting,
  }}
}