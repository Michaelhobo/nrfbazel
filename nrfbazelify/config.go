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

func ReadConfig(sdkDir, workspaceDir string, verbose bool) (*Config, error) {
	conf := &Config{
		SDKDir: sdkDir,
		WorkspaceDir: workspaceDir,
		Verbose: verbose,
		IgnoreHeaders: make(map[string]bool),
		IncludeOverrides: make(map[string]*bazel.Label),
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
}

// Makes a copy of relPaths where all paths will be absolute, prefixed with sdkDir. 
func makeAbs(sdkDir string, relPaths []string) []string {
	out := make([]string, 0, len(relPaths))
	for _, relPath := range relPaths {
		out = append(out, filepath.Join(sdkDir, relPath))
	}
	return out
}