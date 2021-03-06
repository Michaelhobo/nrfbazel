package nrfbazelify

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/Michaelhobo/nrfbazel/internal/buildfile"
	"github.com/Michaelhobo/nrfbazel/proto/bazelifyrc"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/testing/protocmp"
)

var testDataDir = "testdata"

const garbageText = "garbage to remove"

func mustMakeAbs(t *testing.T, dir string) string {
  t.Helper()
  abs, err := filepath.Abs(dir)
  if err != nil {
    t.Fatalf("filepath.Abs(%s): %v", dir, err)
  }
  return abs
}

func setup(t *testing.T, sdkFromWorkspace string) (workspaceDir, sdkDir string) {
  depGraphPath := filepath.Join("/tmp", fmt.Sprintf("%s_depgraph.dot", strings.ReplaceAll(sdkFromWorkspace, "/", "_")))
  if err := os.Remove(depGraphPath); err != nil && !os.IsNotExist(err) {
    t.Fatalf("os.Remove(%q): %v", depGraphPath, err)
  }
  flag.Set("dot_graph_path", depGraphPath)
  t.Cleanup(func() { flag.Set("dot_graph_path", "") })
  workspaceDir = mustMakeAbs(t, testDataDir)
  sdkDir = filepath.Join(workspaceDir, sdkFromWorkspace)
  t.Cleanup(func() {
    removeAllBuildFiles(t, sdkDir)
  })
  return
}

func newBuildFile(dir string, libs []*buildfile.Library, labelSettings []*buildfile.LabelSetting, exportFiles []string) *buildfile.File {
  out := buildfile.New(dir)
  out.AddLoad(&buildfile.Load{
    Source: "@rules_cc//cc:defs.bzl",
    Symbols: []string{"cc_library"},
  })
  for _, lib := range libs {
    out.AddLibrary(lib)
  }
  for _, labelSetting := range labelSettings {
    out.AddLabelSetting(labelSetting)
  }
  for _, f := range exportFiles {
    out.ExportFile(f)
  }
  return out
}

func removeAllBuildFiles(t *testing.T, dir string) {
  t.Helper()
  if err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
    if err != nil {
      return nil
    }
    if info.Name() == "BUILD" {
      if err := os.Remove(path); err != nil {
        return err
      }
    }
    return nil
  }); err != nil {
    t.Fatalf("removeAllBuildFiles(%s): %v", dir, err)
  }
}

// Checks that BUILD files contain the correct contents.
func checkBuildFiles(t *testing.T, files ...*buildfile.File) {
  t.Helper()

  for _, file := range files {
    got, err := os.ReadFile(file.Path)
    if err != nil {
      t.Errorf("Failed to read file %s: %v", file.Path, err)
      continue
    }
    want := file.Generate()
    if diff := cmp.Diff(want, string(got)); diff != "" {
      t.Errorf("%s (-want +got):\n%s", file.Path, diff)
    }
  }
}

func TestGenerateBuildFiles_Nominal(t *testing.T) {
  workspaceDir, sdkDir := setup(t, "nominal")
  if err := GenerateBuildFiles(workspaceDir, sdkDir, true); err != nil {
    t.Fatalf("GenerateBuildFiles(%s, %s): %v", workspaceDir, sdkDir, err)
  }
  checkBuildFiles(t,
    newBuildFile(sdkDir, []*buildfile.Library{
      {
        Name:     "a",
        Hdrs:     []string{"a.h"},
        Deps:     []string{":b"},
        Copts: 		[]string{"-Inominal"},
      },
      {
        Name:     "b",
        Srcs:     []string{"b.c"},
        Hdrs:     []string{"b.h"},
        Deps:     []string{"//nominal/dir:c"},
        Copts: 		[]string{"-Inominal/dir"},
      },
    }, nil, nil),
    newBuildFile(filepath.Join(sdkDir, "dir"), []*buildfile.Library{
      {
        Name:     "c",
        Srcs:     []string{"c.c"},
        Hdrs:     []string{"c.h"},
      },
    }, nil, nil),
  )
}

func TestGenerateBuildFiles_NameMatchesDir(t *testing.T) {
  workspaceDir, sdkDir := setup(t, "name_matches_dir")
  if err := GenerateBuildFiles(workspaceDir, sdkDir, true); err != nil {
    t.Fatalf("GenerateBuildFiles(%s, %s): %v", workspaceDir, sdkDir, err)
  }
  checkBuildFiles(t, 
    newBuildFile(sdkDir, []*buildfile.Library{
      {
        Name:     "uses_dir",
        Hdrs:     []string{"uses_dir.h"},
        Deps:     []string{"//name_matches_dir/dir"},
        Copts: []string{"-Iname_matches_dir/dir"},
      },
    }, nil, nil),
    newBuildFile(filepath.Join(sdkDir, "dir"), []*buildfile.Library{
      {
        Name:     "dir",
        Hdrs:     []string{"dir.h"},
      },
    }, nil, nil),
  )
}

func TestGenerateBuildFiles_RelativeIncludes(t *testing.T) {
  workspaceDir, sdkDir := setup(t, "relative_includes")
  if err := GenerateBuildFiles(workspaceDir, sdkDir, true); err != nil {
    t.Fatalf("GenerateBuildFiles(%s, %s): %v", workspaceDir, sdkDir, err)
  }
  checkBuildFiles(t,
    newBuildFile(filepath.Join(sdkDir, "up_one"), []*buildfile.Library{
      {
        Name:     "a",
        Hdrs:     []string{"a.h"},
        Srcs: 		[]string{"a.c"},
        Deps:     []string{"//relative_includes/back_and_around:b"},
        Copts: []string{"-Irelative_includes/back_and_around"},
      },
    }, nil, nil),
    newBuildFile(filepath.Join(sdkDir, "back_and_around"), []*buildfile.Library{
      {
        Name:     "b",
        Hdrs:     []string{"b.h"},
        Srcs: 		[]string{"b.c"},
      },
    }, nil, nil),
  )
}

func TestGenerateBuildFiles_BuildFileExists(t *testing.T) {
  workspaceDir, sdkDir := setup(t, "build_file_exists")
  garbageBuild := filepath.Join(sdkDir, "BUILD")
  if err := os.WriteFile(garbageBuild, []byte(garbageText), 0644); err != nil {
    t.Fatalf("os.WriteFile(%s, %s): %v", garbageBuild, garbageText, err)
  }
  if err := GenerateBuildFiles(workspaceDir, sdkDir, true); err != nil {
    t.Fatalf("GenerateBuildFiles(%s, %s): %v", workspaceDir, sdkDir, err)
  }
  checkBuildFiles(t,
    newBuildFile(sdkDir, []*buildfile.Library{
      {
        Name:     "a",
        Hdrs:     []string{"a.h"},
      },
    }, nil, nil),
  )
  buildPath := filepath.Join(sdkDir, "BUILD")
  contents, err := os.ReadFile(buildPath)
  if err != nil {
    t.Fatalf("os.ReadFile(%s): %v", buildPath, err)
  }
  if got, dontWant := string(contents), "garbage to remove"; strings.Contains(got, dontWant) {
    t.Errorf("strings.Contains(%s, %s): got true, want false", got, dontWant)
  }
}

func TestGenerateBuildFiles_WorkspaceMatchesSDKDir(t *testing.T) {
  _, workspaceAndSDKDir := setup(t, "workspace_matches_sdk_dir")
  if err := GenerateBuildFiles(workspaceAndSDKDir, workspaceAndSDKDir, true); err != nil {
    t.Fatalf("GenerateBuildFiles(%s, %s): %v", testDataDir, workspaceAndSDKDir, err)
  }
  checkBuildFiles(t, 
    newBuildFile(workspaceAndSDKDir, []*buildfile.Library{
      {
        Name:     "a",
        Hdrs:     []string{"a.h"},
        Deps:     []string{":workspace_matches_sdk_dir"},
        Copts: []string{"-I."},
      },
      {
        Name:     "workspace_matches_sdk_dir",
        Srcs:     []string{"workspace_matches_sdk_dir.c"},
        Hdrs:     []string{"workspace_matches_sdk_dir.h"},
      },
    }, nil, nil),
  )
}

func TestGenerateBuildFiles_IncludeDoesNotExist(t *testing.T) {
  workspaceDir, sdkDir := setup(t, "include_does_not_exist")
  if err := GenerateBuildFiles(workspaceDir, sdkDir, true); err == nil {
    t.Errorf("GenerateBuildFiles(%s, %s): got nil error, want an error", workspaceDir, sdkDir)
  }
}

func TestGenerateBuildFiles_BazelifyRCHint(t *testing.T) {
  workspaceDir, sdkDir := setup(t, "bazelifyrc_hint")
  if err := GenerateBuildFiles(workspaceDir, sdkDir, true); err == nil {
    t.Fatalf("GenerateBuildFiles(%s, %s): got nil error, want an error", workspaceDir, sdkDir)
  }
  hintPath := filepath.Join(sdkDir, ".bazelifyrc.hint")
  hintText, err := os.ReadFile(hintPath)
  if err != nil {
    t.Fatalf("os.ReadFile(%s): %v", hintPath, err)
  }
  var hint bazelifyrc.Configuration 
  if err := prototext.Unmarshal(hintText, &hint); err != nil {
    t.Fatalf("proto.UnmarshalText(%s): %v", string(hintText), err)
  }
  if diff := cmp.Diff(&bazelifyrc.Configuration{
    IncludeOverrides: []*bazelifyrc.IncludeOverride{
      {
				Include: "doesnotexist.h",
				Label: "INCLUDED BY //bazelifyrc_hint:exists PLEASE RESOLVE: ",
			},
    },
  }, hint, protocmp.Transform()); diff != "" {
    t.Fatalf("bazelifyrc hint (-want +got): %s", diff)
  }
}

func TestGenerateBuildFiles_BazelifyRCHintKeepOverride(t *testing.T) {
  workspaceDir, sdkDir := setup(t, "bazelifyrc_hint_keep_override")
  if err := GenerateBuildFiles(workspaceDir, sdkDir, true); err == nil {
    t.Fatalf("GenerateBuildFiles(%s, %s): got nil error, want an error", workspaceDir, sdkDir)
  }
  hintPath := filepath.Join(sdkDir, ".bazelifyrc.hint")
  hintText, err := os.ReadFile(hintPath)
  if err != nil {
    t.Fatalf("os.ReadFile(%s): %v", hintPath, err)
  }
  var hint bazelifyrc.Configuration 
  if err := prototext.Unmarshal(hintText, &hint); err != nil {
    t.Fatalf("proto.UnmarshalText(%s): %v", string(hintText), err)
  }
  if diff := cmp.Diff(&bazelifyrc.Configuration{
    IncludeOverrides: []*bazelifyrc.IncludeOverride{
      {
				Include: "overridden.h",
				Label: "//something",
			},
      {
				Include: "doesnotexist.h",
				Label: "INCLUDED BY //bazelifyrc_hint_keep_override:exists PLEASE RESOLVE: ",
			},
    },
  }, hint, protocmp.Transform()); diff != "" {
    t.Fatalf("bazelifyrc hint (-want +got): %s", diff)
  }
}

func TestGenerateBuildFiles_IncludeOverrides(t *testing.T) {
  workspaceDir, sdkDir := setup(t, filepath.Join("include_overrides", "sdkdir"))
  if err := GenerateBuildFiles(workspaceDir, sdkDir, true); err != nil {
    t.Fatalf("GenerateBuildFiles(%s, %s): %v", workspaceDir, sdkDir, err)
  }
  checkBuildFiles(t, 
    newBuildFile(sdkDir, []*buildfile.Library{
      {
        Name:     "a",
        Hdrs:     []string{"a.h"},
        Copts: []string{
					"-Iinclude_overrides/outsidesdkdir",
					"-Iinclude_overrides/sdkdir/b",
					"-Iinclude_overrides/sdkdir/c",
				},
        Deps: []string{
          "//include_overrides/outsidesdkdir:d",
          "//include_overrides/sdkdir/b",
          "//include_overrides/sdkdir/c",
        },
      },
      {
        Name:     "c",
        Hdrs:     []string{"c.h"},
      },
    }, nil, nil),
    newBuildFile(filepath.Join(sdkDir, "b"), []*buildfile.Library{
      {
        Name:     "b",
        Hdrs:     []string{"b.h"},
      },
    }, nil, nil),
    newBuildFile(filepath.Join(sdkDir, "c"), []*buildfile.Library{
      {
        Name:     "c",
        Hdrs:     []string{"c.h"},
      },
    }, nil, nil),
  )
}

func TestGenerateBuildFiles_BazelifyRCExistsButEmpty(t *testing.T) {
  workspaceDir, sdkDir := setup(t, "bazelifyrc_exists_but_empty")
  if err := GenerateBuildFiles(workspaceDir, sdkDir, true); err != nil {
    t.Fatalf("GenerateBuildFiles(%s, %s): %v", workspaceDir, sdkDir, err)
  }
  checkBuildFiles(t,
    newBuildFile(sdkDir, []*buildfile.Library{
      {
        Name:     "a",
        Hdrs:     []string{"a.h"},
      },
    }, nil, nil),
  )
}

func TestGenerateBuildFiles_StrangeInclude(t *testing.T) {
  workspaceDir, sdkDir := setup(t, "strange_includes")
  if err := GenerateBuildFiles(workspaceDir, sdkDir, true); err != nil {
    t.Fatalf("GenerateBuildFiles(%s, %s): %v", workspaceDir, sdkDir, err)
  }
  checkBuildFiles(t,
    newBuildFile(sdkDir, []*buildfile.Library{
      {
        Name:     "a",
        Hdrs:     []string{"a.h"},
        Copts: []string{"-Istrange_includes"},
        Deps:     []string{":b", ":d"},
      },
      {
        Name:     "b",
        Hdrs:     []string{"b.h"},
      },
      {
        Name:     "d",
        Hdrs:     []string{"d.h"},
      },
    }, nil, nil),
  )
}

func TestGenerateBuildFiles_BazelifyRCExcludes(t *testing.T) {
  workspaceDir, sdkDir := setup(t, "bazelifyrc_excludes")
  if err := GenerateBuildFiles(workspaceDir, sdkDir, true); err != nil {
    t.Fatalf("GenerateBuildFiles(%s, %s): %v", workspaceDir, sdkDir, err)
  }
  checkBuildFiles(t,
    newBuildFile(sdkDir, []*buildfile.Library{
      {
        Name:     "a",
        Hdrs:     []string{"a.h"},
        Copts: []string{
					"-Ibazelifyrc_excludes/included",
					"-Ibazelifyrc_excludes/included/e",
				},
        Deps: []string{
          "//bazelifyrc_excludes/included/e",
          "//bazelifyrc_excludes/included:d",
        },
      },
      {
        Name:     "b",
        Hdrs:     []string{"b.h"},
        Copts: []string{"-Ibazelifyrc_excludes"},
        Deps:     []string{":a"},
      },
    }, nil, nil),
    newBuildFile(filepath.Join(sdkDir, "included"), []*buildfile.Library{
      {
        Name:     "d",
        Hdrs:     []string{"d.h"},
      },
    }, nil, nil),
    newBuildFile(filepath.Join(sdkDir, "included", "e"), []*buildfile.Library{
      {
        Name:     "e",
        Hdrs:     []string{"e.h"},
      },
    }, nil, nil),
  )
  // Make sure BUILD files aren't created for excluded directories
  buildShouldNotExist := []string{
    "excluded",
    "included/star_excluded",
    "included/star_excluded/e",
  }
  for _, dir := range buildShouldNotExist {
    path := filepath.Join(sdkDir, dir, "BUILD")
    if _, err := os.Stat(path); err == nil {
      t.Errorf("BUILD file in %s created, but should not have been created", dir)
    }
  }
}

func TestGenerateBuildFiles_BazelifyRCIgnoreHeaders(t *testing.T) {
  workspaceDir, sdkDir := setup(t, "bazelifyrc_ignore_headers")
  if err := GenerateBuildFiles(workspaceDir, sdkDir, true); err != nil {
    t.Fatalf("GenerateBuildFiles(%s, %s): %v", workspaceDir, sdkDir, err)
  }
  checkBuildFiles(t,
    newBuildFile(sdkDir, []*buildfile.Library{
      {
        Name:     "a",
        Hdrs:     []string{"a.h"},
      },
    }, nil, nil),
  )
}

func TestGenerateBuildFiles_BazelifyRCIncludeDirs(t *testing.T) {
  workspaceDir, sdkDir := setup(t, "bazelifyrc_include_dirs")
  if err := GenerateBuildFiles(workspaceDir, sdkDir, true); err != nil {
    t.Fatalf("GenerateBuildFiles(%s, %s): %v", workspaceDir, sdkDir, err)
  }
  checkBuildFiles(t,
    newBuildFile(sdkDir, []*buildfile.Library{
      {
        Name:     "a",
        Hdrs:     []string{"a.h"},
        Copts: []string{
					"-Ibazelifyrc_include_dirs",
					"-Ibazelifyrc_include_dirs/external",
					"-Ibazelifyrc_include_dirs/external/b",
				},
        Deps: []string{
          "//bazelifyrc_include_dirs/external/b",
          "//bazelifyrc_include_dirs/external:d",
          ":c",
        },
      },
      {
        Name:     "c",
        Hdrs:     []string{"c.h"},
      },
    }, nil, nil),
    newBuildFile(filepath.Join(sdkDir, "external"), []*buildfile.Library{
      {
        Name:     "d",
        Hdrs:     []string{"d.h"},
      },
    }, nil, nil),
    newBuildFile(filepath.Join(sdkDir, "external", "b"), []*buildfile.Library{
      {
        Name:     "b",
        Hdrs:     []string{"b.h"},
      },
    }, nil, nil),
    newBuildFile(filepath.Join(sdkDir, "shouldskipthese"), []*buildfile.Library{
      {
        Name:     "d",
        Hdrs:     []string{"d.h"},
      },
    }, nil, nil),
    newBuildFile(filepath.Join(sdkDir, "shouldskipthese", "b"), []*buildfile.Library{
      {
        Name:     "b",
        Hdrs:     []string{"b.h"},
      },
    }, nil, nil),
    newBuildFile(filepath.Join(sdkDir, "shouldskipthese", "shouldalsoskipthese"), []*buildfile.Library{
      {
        Name:     "d",
        Hdrs:     []string{"d.h"},
      },
    }, nil, nil),
  )
}

func TestGenerateBuildFiles_BazelifyRCMalformed(t *testing.T) {
  workspaceDir, sdkDir := setup(t, "bazelifyrc_malformed")
  if err := GenerateBuildFiles(workspaceDir, sdkDir, true); err == nil {
    t.Fatalf("GenerateBuildFiles(%s, %s): nil error, want an error", workspaceDir, sdkDir)
  }
  hintPath := filepath.Join(sdkDir, ".bazelifyrc.hint")
  if _, err := os.Stat(hintPath); err == nil {
    t.Fatalf("os.Stat(%s): nil error, want an error", hintPath)
  }
}

func TestGenerateBuildFiles_BazelifyRCRemap(t *testing.T) {
  workspaceDir, sdkDir := setup(t, "bazelifyrc_remap")
  if err := GenerateBuildFiles(workspaceDir, sdkDir, true); err != nil {
    t.Fatalf("GenerateBuildFiles(%s, %s): %v", workspaceDir, sdkDir, err)
  }
  checkBuildFiles(t,
    newBuildFile(sdkDir, []*buildfile.Library{
      {
        Name: "nrfbazelify_empty_remap",
      },
      {
        Name:     "a",
        Hdrs:     []string{"a.h"},
      },
      {
        Name:     "b",
        Hdrs:     []string{"b.h"},
      },
      {
        Name:     "c",
        Hdrs:     []string{"c.h"},
        Deps:     []string{
          ":a_remap",
          ":b_remap",
        },
      },
    }, []*buildfile.LabelSetting{
      {
        Name: "a_remap",
        BuildSettingDefault: "//bazelifyrc_remap:nrfbazelify_empty_remap",
      },
      {
        Name: "b_remap",
        BuildSettingDefault: "//bazelifyrc_remap:nrfbazelify_empty_remap",
      },
    }, nil),
  )

  remapBzl, err := os.ReadFile(filepath.Join(sdkDir, "remap.bzl"))
  if err != nil {
    t.Fatalf("read remap.bzl: %v", err)
  }
  t.Logf("remap.bzl contents:\n%s", string(remapBzl))
  searchPhrases := map[string]string{
    "remapTransitionReturnsA": "\"//bazelifyrc_remap:a_remap\": attr.a,",
    "remapTransitionReturnsB": "\"//bazelifyrc_remap:b_remap\": attr.b,",
    "remapTransitionOutputsA": "\"//bazelifyrc_remap:a_remap\",",
    "remapTransitionOutputsB": "\"//bazelifyrc_remap:b_remap\",",
  }
  for name, phrase := range searchPhrases {
    t.Run(name, func(t *testing.T) {
      match, err := regexp.MatchString(phrase, string(remapBzl))
      if err != nil {
        t.Errorf("regexp.MatchString: %v", err)
        return
      }
      if !match {
        t.Errorf("phrase not found:\n%s", phrase)
      }
    })
  }
}

func TestGenerateBuildFiles_RemovesStaleHint(t *testing.T) {
  workspaceDir, sdkDir := setup(t, "removes_stale_hint")
  if err := GenerateBuildFiles(workspaceDir, sdkDir, true); err != nil {
    t.Fatalf("GenerateBuildFiles(%s, %s): %v", workspaceDir, sdkDir, err)
  }
  hintFile := filepath.Join(sdkDir, ".bazelifyrc.hint")
  if _, err := os.Stat(hintFile); err == nil {
    t.Errorf("hint file %q not removed after successful run", hintFile)
  }
}

func TestGeneratedBuildFiles_SourceSets(t *testing.T) {
  workspaceDir, sdkDir := setup(t, "source_sets")
  if err := GenerateBuildFiles(workspaceDir, sdkDir, true); err != nil {
    t.Fatalf("GenerateBuildFiles(%s, %s): %v", workspaceDir, sdkDir, err)
  }
  checkBuildFiles(t,
    newBuildFile(sdkDir, []*buildfile.Library{
      {
        Name:     "ab",
        Hdrs:     []string{"a.h", "b.h"},
        Srcs:     []string{"b.c"},
        Copts: []string{"-Isource_sets/dir"},
        Deps: []string{
          "//source_sets/dir:c",
        },
      },
    }, nil, nil),
    newBuildFile(filepath.Join(sdkDir, "dir"), []*buildfile.Library{
      {
        Name:     "c",
        Hdrs:     []string{"c.h"},
      },
    }, nil, nil),
  )
}

func TestGenerateBuildFiles_CyclesNominal(t *testing.T) {
  workspaceDir, sdkDir := setup(t, "cycles_nominal")
  if err := GenerateBuildFiles(workspaceDir, sdkDir, true); err != nil {
    t.Fatalf("GenerateBuildFiles(%s, %s): %v", workspaceDir, sdkDir, err)
  }
  checkBuildFiles(t,
    newBuildFile(sdkDir, []*buildfile.Library{
      {
        Name: "a",
        Deps: []string{":abcd"},
      },
      {
        Name: "abcd",
        Hdrs: []string{
          "//cycles_nominal/dir2:d.h",
          "//cycles_nominal/dir:c.h",
          "a.h",
          "b.h",
        },
        Copts: []string{
					"-Icycles_nominal",
					"-Icycles_nominal/dir",
					"-Icycles_nominal/dir2",
				},
        Deps: []string{"//cycles_nominal/dir2:used_by_cyclic"},
      },
      {
        Name: "b",
        Deps: []string{":abcd"},
      },
    }, nil, nil),
    newBuildFile(filepath.Join(sdkDir, "dir"), []*buildfile.Library{
      {
        Name: "uses_cyclic",
        Hdrs: []string{"uses_cyclic.h"},
        Copts: []string{"-Icycles_nominal/dir"},
        Deps: []string{":c"},
      },
      {
        Name: "c",
        Deps: []string{"//cycles_nominal:abcd"},
      },
    }, nil, []string{"c.h"}),
    newBuildFile(filepath.Join(sdkDir, "dir2"), []*buildfile.Library{
      {
        Name: "used_by_cyclic",
        Hdrs: []string{"used_by_cyclic.h"},
      },
      {
        Name: "d",
        Deps: []string{"//cycles_nominal:abcd"},
      },
    }, nil, []string{"d.h"}),
  )
}

func TestGenerateBuildFiles_CyclesMultipleGroups(t *testing.T) {
  workspaceDir, sdkDir := setup(t, "cycles_multiple_groups")
  if err := GenerateBuildFiles(workspaceDir, sdkDir, true); err != nil {
    t.Fatalf("GenerateBuildFiles(%s, %s): %v", workspaceDir, sdkDir, err)
  }
  checkBuildFiles(t,
    newBuildFile(sdkDir, []*buildfile.Library{
      {
        Name: "big_group",
        Hdrs: []string{
          "//cycles_multiple_groups/group1:a.h",
          "//cycles_multiple_groups/group1:b.h",
          "//cycles_multiple_groups/group1:c.h",
          "//cycles_multiple_groups/group2:d.h",
          "//cycles_multiple_groups/group2:e.h",
          "//cycles_multiple_groups/group2:f.h",
          "middle1.h",
          "middle2.h",
          "middle3.h",
          "middle4.h",
          "middle5.h",
          "middle6.h",
        },
        Copts: []string{
          "-Icycles_multiple_groups",
          "-Icycles_multiple_groups/group1",
          "-Icycles_multiple_groups/group2",
        },
      },
      {
        Name: "middle1",
        Deps: []string{":big_group"},
      },
      {
        Name: "middle2",
        Deps: []string{":big_group"},
      },
      {
        Name: "middle3",
        Deps: []string{":big_group"},
      },
      {
        Name: "middle4",
        Deps: []string{":big_group"},
      },
      {
        Name: "middle5",
        Deps: []string{":big_group"},
      },
      {
        Name: "middle6",
        Deps: []string{":big_group"},
      },
    }, nil, nil),
    newBuildFile(filepath.Join(sdkDir, "group1"), []*buildfile.Library{
      {
        Name: "a",
        Deps: []string{"//cycles_multiple_groups:big_group"},
      },
      {
        Name: "b",
        Deps: []string{"//cycles_multiple_groups:big_group"},
      },
      {
        Name: "c",
        Deps: []string{"//cycles_multiple_groups:big_group"},
      },
    }, nil, []string{"a.h", "b.h", "c.h"}),
    newBuildFile(filepath.Join(sdkDir, "group2"), []*buildfile.Library{
      {
        Name: "d",
        Deps: []string{"//cycles_multiple_groups:big_group"},
      },
      {
        Name: "e",
        Deps: []string{"//cycles_multiple_groups:big_group"},
      },
      {
        Name: "f",
        Deps: []string{"//cycles_multiple_groups:big_group"},
      },
    }, nil, []string{"d.h", "e.h", "f.h"}),
  )
}