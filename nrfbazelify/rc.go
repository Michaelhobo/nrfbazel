package nrfbazelify

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"github.com/Michaelhobo/nrfbazel/proto/bazelifyrc"
	"github.com/golang/protobuf/proto"
)

func ReadBazelifyRC(sdkDir string) (*bazelifyrc.Configuration, error) {
  // We read this file from the root of the SDK, so that we can have
  // per-SDK overrides in the same workspace.
  rcPath := filepath.Join(sdkDir, rcFilename)
  if _, err := os.Stat(rcPath); err != nil {
    log.Printf("WARNING: No .bazelifyrc found: os.Stat: %v", err)
    return nil, nil
  }
  rcData, err := ioutil.ReadFile(rcPath)
  if err != nil {
    return nil, fmt.Errorf("Could not read %s: %v", rcFilename, err)
  }
  var rc bazelifyrc.Configuration
  if err := proto.UnmarshalText(string(rcData), &rc); err != nil {
    return nil, err
  }
	return &rc, nil
}