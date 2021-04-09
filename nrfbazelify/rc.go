package nrfbazelify

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/Michaelhobo/nrfbazel/proto/bazelifyrc"
	"google.golang.org/protobuf/encoding/prototext"
)

func ReadBazelifyRC(sdkDir string) (*bazelifyrc.Configuration, error) {
  // We read this file from the root of the SDK, so that we can have
  // per-SDK overrides in the same workspace.
  rcPath := filepath.Join(sdkDir, rcFilename)
  if _, err := os.Stat(rcPath); err != nil {
    log.Printf("WARNING: No .bazelifyrc found: os.Stat: %v", err)
    return nil, nil
  }
  rcData, err := os.ReadFile(rcPath)
  if err != nil {
    return nil, fmt.Errorf("could not read %s: %v", rcFilename, err)
  }
  var rc bazelifyrc.Configuration
  if err := prototext.Unmarshal(rcData, &rc); err != nil {
    return nil, err
  }
	return &rc, nil
}