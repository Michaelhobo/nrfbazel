package nrfbazelify

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
  Srcs []*bazel.Label
  Hdrs []*bazel.Label
}

type IncludeOverride struct {
	Label *bazel.Label
	// These IncludeDirs will be added to copts of any rules that depend on it.
	IncludeDirs []string
}

func ReadConfig(sdkDir, workspaceDir string, verbose bool) (*Config, error) {
  conf := &Config{
    SDKDir: sdkDir,
    WorkspaceDir: workspaceDir,
    Verbose: verbose,
    IgnoreHeaders: make(map[string]bool),
    IncludeOverrides: make(map[string]*IncludeOverride),
    SourceSetsByFile: make(map[string]*bazel.Label),
    SourceSets: make(map[string]*CCFiles),
    NamedGroups: make(map[string]map[string]string),
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
    return fmt.Errorf(".bazelifyrc not found: %v\nMake sure this is the right SDK path, or create an empty .bazelifyrc file at the root of the nrf52 SDK", err)
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

  for _, override := range rc.GetIncludeOverrides() {
    label, err := bazel.ParseLabel(override.GetLabel())
    if err != nil {
      return err
    }
    conf.IncludeOverrides[override.GetInclude()] = &IncludeOverride{
			Label: label,
			IncludeDirs: override.GetIncludeDirs(),
		}
  }

  for _, sourceSet := range rc.GetSourceSets() {
    sourceSetDir := filepath.Join(conf.SDKDir, sourceSet.GetDir())
    label, err := bazel.NewLabel(sourceSetDir, sourceSet.GetName(), conf.WorkspaceDir)
    if err != nil {
      return fmt.Errorf("bazel.NewLabel(%v, %v): %v", sourceSetDir, sourceSet.GetName(), err)
    }

    absSrcs := makeAbs(filepath.Join(conf.SDKDir, sourceSet.GetDir()), sourceSet.GetSrcs())
    absHdrs := makeAbs(filepath.Join(conf.SDKDir, sourceSet.GetDir()), sourceSet.GetHdrs())

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
    srcs, err := makeLabels(conf.WorkspaceDir, absSrcs)
    if err != nil {
      return fmt.Errorf("makeLabels(%v): %v", absSrcs, err)
    }
    hdrs, err := makeLabels(conf.WorkspaceDir, absHdrs)
    if err != nil {
      return fmt.Errorf("makeLabels(%v): %v", absHdrs, err)
    }
    conf.SourceSets[label.String()] = &CCFiles{
      Srcs: srcs,
      Hdrs: hdrs,
    }
  }

  // Add named groups.
  for _, namedGroup := range rc.GetNamedGroups() {
    if conf.NamedGroups[namedGroup.GetFirstHdr()] == nil {
      conf.NamedGroups[namedGroup.GetFirstHdr()] = make(map[string]string)
    }
    conf.NamedGroups[namedGroup.GetFirstHdr()][namedGroup.GetLastHdr()] = namedGroup.GetName()
  }

  return nil
}

// BazelifyRC contains validated data from the .bazelifyrc file.
type Config struct {
  SDKDir, WorkspaceDir string
  Verbose bool
  BazelifyRCProto *bazelifyrc.Configuration
  Remaps *remap.Remaps
  Excludes []string // file paths to exclude, converted to absolute paths
  IncludeDirs []string // all paths converted to absolute paths
  IgnoreHeaders map[string]bool // header file name -> should ignore
  IncludeOverrides map[string]*IncludeOverride // file name -> override info
  SourceSetsByFile map[string]*bazel.Label // file path -> label of rule containing file
  SourceSets map[string]*CCFiles // label.String() -> files in source set
  NamedGroups map[string]map[string]string // first header -> last header -> name
}

// Makes a copy of relPaths where all paths will be absolute, prefixed with sdkDir. 
func makeAbs(dir string, relPaths []string) []string {
  out := make([]string, 0, len(relPaths))
  for _, relPath := range relPaths {
    out = append(out, filepath.Join(dir, relPath))
  }
  return out
}

// makeLabels turns the absolute paths into labels.
func makeLabels(workspaceDir string, absPaths []string) ([]*bazel.Label, error) {
  var out []*bazel.Label
  for _, p := range absPaths {
    if !strings.HasPrefix(p, workspaceDir) {
      return nil, fmt.Errorf("%q must be in %q", p, workspaceDir)
    }
    name := filepath.Base(p)
    dir := filepath.Dir(p)
    label, err := bazel.NewLabel(dir, name, workspaceDir)
    if err != nil {
      return nil, fmt.Errorf("bazel.NewLabel(%q, %q, %q): %v", dir, name, workspaceDir, err)
    }
    out = append(out, label)
  }
  return out, nil
}