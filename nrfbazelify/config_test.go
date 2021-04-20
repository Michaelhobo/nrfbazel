package nrfbazelify

import (
	"path/filepath"
	"testing"
)

func TestReadConfig_MissingBazelifyrc(t *testing.T) {
  workspaceDir := mustMakeAbs(t, testDataDir)
  sdkDir := filepath.Join(workspaceDir, "config_missing_bazelifyrc")
  if _, err := ReadConfig(sdkDir, workspaceDir, true); err == nil {
    t.Errorf("ReadConfig: want an error")
  }
}
