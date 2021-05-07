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

	// Add the nrf_defines setting, used for propagating GCC defines to all libs.
	sdkRelDir, err := filepath.Rel(conf.WorkspaceDir, conf.SDKDir)
	if err != nil {
		return fmt.Errorf("filepath.Rel(%q, %q): %v", conf.WorkspaceDir, conf.SDKDir, err)
	}
	files[sdkRelDir] = buildfile.New(conf.SDKDir)

	files[sdkRelDir].AddLoad(&buildfile.Load{
		Source: "@bazel_skylib//rules:common_settings.bzl",
		Symbols: []string{"string_list_setting"},
	})

	nrfDefinesName := "nrf_defines"
	files[sdkRelDir].AddStringListSetting(&buildfile.StringListSetting{
		Name: nrfDefinesName,
	})
	nrfDefinesLabel, err := bazel.NewLabel(conf.SDKDir, nrfDefinesName, conf.WorkspaceDir)
	if err != nil {
		return fmt.Errorf("bazel.NewLabel(%q, %q): %v", conf.SDKDir, nrfDefinesName, err)
	}

  // Convert depGraph nodes into BUILD files.
  nodes := depGraph.Nodes()
  for _, node := range nodes {
    contents, err := extractBuildContents(node, depGraph, nrfDefinesLabel)
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

func extractBuildContents(node Node, depGraph *DependencyGraph, nrfDefinesLabel *bazel.Label) ([]*buildContents, error) {
  switch n := node.(type) {
  case *LibraryNode:
    return libraryContents(n, depGraph, nrfDefinesLabel), nil
  case *GroupNode:
    return groupContents(n, depGraph, nrfDefinesLabel), nil
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

func libraryContents(node *LibraryNode, depGraph *DependencyGraph, nrfDefinesLabel *bazel.Label) []*buildContents {
  return []*buildContents{{
    dir: node.Label().Dir(),
    library: makeLibrary(node.Label(), node.Srcs, node.Hdrs, nrfDefinesLabel, depGraph),
  }}
}

func groupContents(node *GroupNode, depGraph *DependencyGraph, nrfDefinesLabel *bazel.Label) []*buildContents {
  out := []*buildContents{{
    dir: node.Label().Dir(),
    library: makeLibrary(node.Label(), node.Srcs, node.Hdrs, nrfDefinesLabel, depGraph),
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

// makeLibrary creates a deterministic buildfile.Library by sorting all fields.
func makeLibrary(label *bazel.Label, srcs, hdrs []*bazel.Label, nrfDefinesLabel *bazel.Label, depGraph *DependencyGraph) *buildfile.Library {
  var deps []string
  depNodes := depGraph.Dependencies(label)
  for _, d := range depNodes {
    deps = append(deps, d.Label().RelativeTo(label))
  }

  // Process srcs, hdrs, and copts
  var outSrcs, outHdrs, copts []string
  for _, src := range srcs {
    outSrcs = append(outSrcs, src.FileRelativeTo(label.Dir()))
  }
  for _, hdr := range hdrs {
    outHdrs = append(outHdrs, hdr.FileRelativeTo(label.Dir()))
  }

	// Add -I<include path> to copts for all dependencies.
	copts = append(copts, includesAsCopts(label, hdrs, depGraph)...)

  // Sort the srcs, hdrs, copts, and deps so output has a deterministic order.
  sort.Strings(outSrcs)
  sort.Strings(outHdrs)
  sort.Strings(deps)
  sort.Strings(copts)

	return &buildfile.Library{
		Name: label.Name(),
		Srcs: outSrcs,
		Hdrs: outHdrs,
		Deps: deps,
		Copts: copts,
		DefinesLists: []string{nrfDefinesLabel.RelativeTo(label)},
	}
}

// includesAsCopts finds all includes of all dependencies and headers of a node.
// Dependencies get all their include dirs added.
// If headers are in more than 1 directory, all header directories also get added.
// All includes are returned in the form -I<include path>,
// which is suitable for passing into a cc_library's copts field.
func includesAsCopts(label *bazel.Label, hdrs []*bazel.Label, depGraph *DependencyGraph) []string {
	// Prevent duplicates by using a set.
	includesSet := make(map[string]bool)

	// Add all dependencies' include directories to the includes.
	deps := depGraph.Dependencies(label)
	for _, dep := range deps {
		var includes []string
		switch d := dep.(type) {
		case *LibraryNode:
			includes = d.Includes
		case *OverrideNode:
			includes = d.Includes
		default:
			continue
		}
		for _, include := range includes {
			includesSet[include] = true
		}
	}

	// If headers are part of more than 1 directory,
	// add all their directories to the includes.
	hdrDirsSet := make(map[string]bool)
	for _, hdr := range hdrs {
		hdrDirsSet[hdr.Dir()] = true
	}
	if len(hdrDirsSet) > 1 {
		for hdrDir := range hdrDirsSet {
			includesSet[hdrDir] = true
		}
	}

	// Turn them all into copts-compatible format.
	var out []string
	for include := range includesSet {
		out = append(out, fmt.Sprintf("-I%s", include))
	}
	return out
}

func remapContents(node *RemapNode, depGraph *DependencyGraph) []*buildContents {
  return []*buildContents{{
    dir: node.Label().Dir(),
    labelSetting: node.LabelSetting,
  }}
}