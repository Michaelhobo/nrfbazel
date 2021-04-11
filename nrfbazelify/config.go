package nrfbazelify

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/Michaelhobo/nrfbazel/internal/bazel"
	"github.com/Michaelhobo/nrfbazel/internal/remap"
	"github.com/Michaelhobo/nrfbazel/proto/bazelifyrc"
	"google.golang.org/protobuf/encoding/prototext"
)

const (
  // We read this file from the root of the SDK.
  rcFilename = ".bazelifyrc"
)

type CCFiles struct {
  Srcs []string
  Hdrs []string
}

func ReadConfig(sdkDir, workspaceDir string, verbose bool) (*Config, error) {
  conf := &Config{
    SDKDir: sdkDir,
    WorkspaceDir: workspaceDir,
    Verbose: verbose,
    IgnoreHeaders: make(map[string]bool),
    IncludeOverrides: make(map[string]*bazel.Label),
    SourceSetsByFile: make(map[string]*bazel.Label),
    SourceSets: make(map[string]*CCFiles),
  }
  if err := readBazelifyRC(conf); err != nil {
    return nil, err
  }
  return conf, nil
}

func readBazelifyRC(conf *Config) error {
  // We read this file from the root of the SDK, so that we can have
  // per-SDK overrides in the same workspace.
  rcPath := filepath.Join(conf.SDKDir, rcFilename)
  if _, err := os.Stat(rcPath); err != nil {
    log.Printf("WARNING: No .bazelifyrc found: os.Stat: %v", err)
    return nil
  }
  rcData, err := os.ReadFile(rcPath)
  if err != nil {
    return fmt.Errorf("could not read %s: %v", rcFilename, err)
  }
  var rc bazelifyrc.Configuration
  if err := prototext.Unmarshal(rcData, &rc); err != nil {
    return err
  }

  conf.BazelifyRCProto = &rc

  // Validate and turn proto data into a friendlier format.
  sdkFromWorkspace, err := filepath.Rel(conf.WorkspaceDir, conf.SDKDir)
  if err != nil {
    return fmt.Errorf("filepath.Rel: %v", err)
  }
  remaps, err := remap.New(rc.GetRemaps(), sdkFromWorkspace)
  if err != nil {
    return fmt.Errorf("remap.New: %v", err)
  }
  conf.Remaps = remaps

  conf.Excludes = makeAbs(conf.SDKDir, rc.GetExcludes())

  conf.IncludeDirs = makeAbs(conf.SDKDir, rc.GetIncludeDirs())

  for _, ignore := range rc.GetIgnoreHeaders() {
    conf.IgnoreHeaders[ignore] = true
  }

  for name, override := range rc.GetTargetOverrides() {
    label, err := bazel.ParseLabel(override)
    if err != nil {
      return err
    }
    conf.IncludeOverrides[name] = label
  }

  for _, sourceSet := range rc.GetSourceSets() {
    sourceSetDir := filepath.Join(conf.SDKDir, sourceSet.GetDir())
    label, err := bazel.NewLabel(sourceSetDir, sourceSet.GetName(), conf.WorkspaceDir)
    if err != nil {
      return fmt.Errorf("bazel.NewLabel(%v, %v): %v", sourceSetDir, sourceSet.GetName(), err)
    }

    absSrcs := makeAbs(conf.SDKDir, sourceSet.GetSrcs())
    absHdrs := makeAbs(conf.SDKDir, sourceSet.GetHdrs())

    // Add files to index by file name, and make sure the files exist.
    files := make([]string, 0, len(sourceSet.GetSrcs()) + len(sourceSet.GetHdrs()))
    files = append(files, absSrcs...)
    files = append(files, absHdrs...)
    for _, file := range files {
      if info, err := os.Stat(file); err != nil {
        return fmt.Errorf("os.Stat(%v): %v", file, err)
      } else if info.IsDir() {
        return fmt.Errorf("source set %q contains %q which is a directory", label, file)
      }
      conf.SourceSetsByFile[file] = label
    }

    // Add files to source sets by label.
    // We make the srcs and hdrs relative to the label's directory.
    srcs, err := makeRel(filepath.Join(conf.WorkspaceDir, label.Dir()), absSrcs)
    if err != nil {
      return fmt.Errorf("makeRel(%v): %v", absSrcs, err)
    }
    hdrs, err := makeRel(filepath.Join(conf.WorkspaceDir, label.Dir()), absHdrs)
    if err != nil {
      return fmt.Errorf("makeRel(%v): %v", absHdrs, err)
    }
    conf.SourceSets[label.String()] = &CCFiles{
      Srcs: srcs,
      Hdrs: hdrs,
    }
  }

  return nil
}

// BazelifyRC contains validated data from the .bazelifyrc file.
type Config struct {
  SDKDir, WorkspaceDir string
  Verbose bool
  BazelifyRCProto *bazelifyrc.Configuration
  Remaps *remap.Remaps
  Excludes []string // header file names
  IncludeDirs []string // all paths converted to absolute paths
  IgnoreHeaders map[string]bool // header file name -> should ignore
  IncludeOverrides map[string]*bazel.Label // file name -> override label
  SourceSetsByFile map[string]*bazel.Label // file path -> label of rule containing file
  SourceSets map[string]*CCFiles // label.String() -> files in source set
}

// Makes a copy of relPaths where all paths will be absolute, prefixed with sdkDir. 
func makeAbs(dir string, relPaths []string) []string {
  out := make([]string, 0, len(relPaths))
  for _, relPath := range relPaths {
    out = append(out, filepath.Join(dir, relPath))
  }
  return out
}

// makeRel makes a copy of absPaths where all paths are relative from dir.
// Any current directory paths (aka ".") will turn into empty strings.
func makeRel(dir string, absPaths []string) ([]string, error) {
  out := make([]string, 0, len(absPaths))
  for _, absPath := range absPaths {
    relPath, err := filepath.Rel(dir, absPath)
    if err != nil {
      return nil, fmt.Errorf("filepath.Rel(%q, %q): %v", dir, absPath, err)
    }
    if relPath == "." {
      relPath = ""
    }
    out = append(out, relPath)
  }
  return out, nil
}