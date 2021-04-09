package nrfbazelify

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/Michaelhobo/nrfbazel/proto/bazelifyrc"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
)

// WriteNewHint writes a new bazelifyrc hint file that contains hints for unresolved dependencies.
func WriteNewHint(unresolved []*unresolvedDep, rc *bazelifyrc.Configuration, sdkDir string, verbose bool) error {
	hint := unresolvedDepsHint(unresolved, rc)
	rcPath := filepath.Join(sdkDir, rcFilename)
	rcHintPath := rcPath + ".hint"
	verboseText := ""
	if verbose {
		verboseText = fmt.Sprintf("\n.bazelifyrc.hint contents:\n%s", string(hint))
	}
	if err := os.WriteFile(rcHintPath, []byte(hint), 0640); err != nil {
		return fmt.Errorf("found unresolved targets. Failed to write hint file: %v%s", err, verboseText)
	}
	return fmt.Errorf("found unresolved target.\nPlease add the resolutions to %s and try again.\nHint written to %s%s", rcPath, rcHintPath, verboseText)
}

func RemoveStaleHint(sdkDir string) error {
  hintFile := filepath.Join(sdkDir, fmt.Sprintf("%s.hint", rcFilename))
  if err := os.Remove(hintFile); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
	
}

func unresolvedDepsHint(unresolved []*unresolvedDep, rc *bazelifyrc.Configuration) []byte {
  rc = proto.Clone(rc).(*bazelifyrc.Configuration)
  if rc == nil {
    rc = &bazelifyrc.Configuration{}
  }
  if rc.GetTargetOverrides() == nil {
    rc.TargetOverrides = make(map[string]string)
  }
  for _, dep := range unresolved {
		var includedBy []string
		for _, label := range dep.includedBy {
			includedBy = append(includedBy, label.String())
		}
		var pleaseResolve []string
		for _, label := range dep.possible {
			pleaseResolve = append(pleaseResolve, label.String())
		}
		possibilities := fmt.Sprintf("INCLUDED BY %s PLEASE RESOLVE: %s", strings.Join(includedBy, ","), strings.Join(pleaseResolve, "|"))
		rc.TargetOverrides[dep.dstFileName] = possibilities
  }
  out, err := prototext.Marshal(rc)
  if err != nil {
    log.Fatalf("prototext.Marshal bazelifyrc hint: %v", err)
  }
  return out
}