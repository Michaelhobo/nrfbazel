package nrfbazelify

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Michaelhobo/nrfbazel/proto/bazelifyrc"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
)

// WriteUnresolvedDepsHint writes a new bazelifyrc hint file that contains hints for unresolved dependencies.
func WriteUnresolvedDepsHint(conf *Config, unresolved []*unresolvedDep) error {
  hint := unresolvedDepsHint(conf, unresolved)
	return writeHintFileErrorf(conf, hint, "found unresolved targets.")
}

func WriteUnnamedGroupsHint(conf *Config, unnamed []*GroupNode) error {
	hint := unnamedGroupsHint(conf, unnamed)
	return writeHintFileErrorf(conf, hint, "found grouped rules that haven't been named.")
}

func RemoveStaleHint(sdkDir string) error {
  hintFile := filepath.Join(sdkDir, fmt.Sprintf("%s.hint", rcFilename))
  if err := os.Remove(hintFile); err != nil && !os.IsNotExist(err) {
    return err
  }
  return nil
}

// Write the hint to the .bazelifyrc hint file, and returns an error message to
// prompt the user to look at the hint file.
// Appends the given msg to the error.
func writeHintFileErrorf(conf *Config, hint []byte, msg string) error {
  rcPath := filepath.Join(conf.SDKDir, rcFilename)
  rcHintPath := rcPath + ".hint"
  verboseText := ""
  if conf.Verbose {
    verboseText = fmt.Sprintf("\n.bazelifyrc.hint contents:\n%s", string(hint))
  }
  if err := os.WriteFile(rcHintPath, []byte(hint), 0640); err != nil {
    return fmt.Errorf("%s\nFailed to write hint file: %v%s", msg, err, verboseText)
  }
	return fmt.Errorf("%s\nPlease add the resolutions to %s and try again.\nHint written to %s%s", msg, rcPath, rcHintPath, verboseText)
}

func unresolvedDepsHint(conf *Config, unresolved []*unresolvedDep) []byte {
  rc := proto.Clone(conf.BazelifyRCProto).(*bazelifyrc.Configuration)
  if rc == nil {
    rc = &bazelifyrc.Configuration{}
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
    rc.IncludeOverrides = append(rc.IncludeOverrides, &bazelifyrc.IncludeOverride{
			Include: dep.dstFileName,
			Label: possibilities,
		})
  }
  out, err := (&prototext.MarshalOptions{
    Multiline: true,
  }).Marshal(rc)
  if err != nil {
    log.Fatalf("prototext.Marshal bazelifyrc hint: %v", err)
  }
  return out
}

func unnamedGroupsHint(conf *Config, unnamed []*GroupNode) []byte {
  rc := proto.Clone(conf.BazelifyRCProto).(*bazelifyrc.Configuration)
  if rc == nil {
    rc = &bazelifyrc.Configuration{}
  }
	for _, node := range unnamed {
		var hdrs []string
		for _, hdr := range node.Hdrs {
			hdrs = append(hdrs, hdr.String())
		}
		sort.Strings(hdrs)
		rc.NamedGroups = append(rc.NamedGroups, &bazelifyrc.NamedGroup{
			Name: node.Label().Name(),
			FirstHdr: hdrs[0],
			LastHdr: hdrs[len(hdrs) - 1],
		})
	}
  out, err := (&prototext.MarshalOptions{
    Multiline: true,
  }).Marshal(rc)
  if err != nil {
    log.Fatalf("prototext.Marshal bazelifyrc hint: %v", err)
  }
  return out
}
