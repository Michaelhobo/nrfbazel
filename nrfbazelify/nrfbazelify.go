// Package nrfbazelify converts nRF5 SDKs to use Bazel.
// This contains a Bazel BUILD file generator that reads source files
// and does the heavy lifting of resolving targets and writing BUILD files.
package nrfbazelify

import (
	"bufio"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/Michaelhobo/nrfbazel/internal/bazel"
	"github.com/Michaelhobo/nrfbazel/internal/buildfile"
	"github.com/Michaelhobo/nrfbazel/internal/remap"
	"github.com/golang/protobuf/proto"

	"github.com/Michaelhobo/nrfbazel/proto/bazelifyrc"
)

const (
  // We read this file from the root of the SDK.
  rcFilename = ".bazelifyrc"
  // We write the contents of our remap features to this file.
  bzlFilename = "remap.bzl"
)

var includeMatcher = regexp.MustCompile("^\\s*#include\\s+\"(.+)\".*$")

// GenerateBuildFiles generates BUILD files for all C source files in sdkDir
// and marks all includes starting from workspaceDir.
func GenerateBuildFiles(workspaceDir, sdkDir string, verbose bool) error {
  if !filepath.IsAbs(workspaceDir) {
    return errors.New("workspace must be an absolute path")
  }
  if !filepath.IsAbs(sdkDir) {
    return errors.New("sdk must be an absolute path")
  }
  if !strings.HasPrefix(sdkDir, workspaceDir) {
    return fmt.Errorf("sdk_dir is not inside workspace_dir:\nsdk_dir=%s\nworkspace_dir=%s", sdkDir, workspaceDir)
  }
  log.Printf("Generating BUILD files for %s", sdkDir)
  gen := &buildGen{
    workspaceDir: filepath.Clean(workspaceDir),
    verbose: 			verbose,
    sdkDir:       filepath.Clean(sdkDir),
    targets:      make(map[string]*possibleTargets),
    sourceSets:   make(map[string]map[string]*bazel.Label),
  }
  return gen.generate()
}

// buildGen generates Bazel BUILD files for nrf SDKs.
type buildGen struct {
  // These are pre-cleaned by GenerateBuildFiles
  workspaceDir, sdkDir string
  verbose bool
  targets              map[string]*possibleTargets // include name -> all possible targets
  sourceSets           map[string]map[string]*bazel.Label // dir (sdk-relative) -> header name -> target label
  // Additional targets that need dependencies resolved, but the library itself is already set.
  sourceSetTargets     []*sourceSetTarget
  rc                   *bazelifyrc.Configuration
}

func (b *buildGen) generate() error {
  if err := b.loadBazelifyRC(); err != nil {
    return fmt.Errorf("not loading .bazelifyrc: %v", err)
  }
  if err := filepath.Walk(b.sdkDir, b.buildTargetsMap); err != nil {
    return fmt.Errorf("filepath.Walk(%s): %v", b.sdkDir, err)
  }
  if err := b.walkSourceSets(); err != nil {
    return fmt.Errorf("failed to walk source sets: %v", err)
  }
  if err := b.checkResolvable(); err != nil {
    return fmt.Errorf("failed to resolve targets: %v", err)
  }
  if err := b.outputFiles(); err != nil {
    return fmt.Errorf("failed to output BUILD files: %v", err)
  }
  if err := b.removeStaleHint(); err != nil {
    return fmt.Errorf("failed to remove stale hint file: %v", err)
  }
  return nil
}

func (b *buildGen) loadBazelifyRC() error {
  // We read this file from the root of the SDK, so that we can have
  // per-SDK overrides in the same workspace.
  rcPath := filepath.Join(b.sdkDir, rcFilename)
  if _, err := os.Stat(rcPath); err != nil {
    log.Printf("WARNING: No .bazelifyrc found: os.Stat: %v", err)
    return nil
  }
  rcData, err := ioutil.ReadFile(rcPath)
  if err != nil {
    return fmt.Errorf("could not read %s: %v", rcFilename, err)
  }
  var rc bazelifyrc.Configuration
  if err := proto.UnmarshalText(string(rcData), &rc); err != nil {
    return err
  }
  b.rc = &rc
  for name, override := range rc.GetTargetOverrides() {
    if b.targets[name] != nil {
      return fmt.Errorf("duplicate target override for %q in %s", name, rcFilename)
    }
    overrideLabel, err := bazel.ParseLabel(override)
    if err != nil {
      return fmt.Errorf("bazel.ParseLabel(%q): %v", override, err)
    }
    b.targets[name] = &possibleTargets{
      override: overrideLabel,
    }
  }
  for _, r := range rc.GetRemaps() {
    if filepath.Ext(r) != ".h" {
      return fmt.Errorf("invalid remap %q: must have .h extension", r)
    }
    if b.targets[r] == nil {
      b.targets[r] = &possibleTargets{}
    }
    if b.targets[r].override == nil {
      remapLabel, err := remap.GenerateLabel(r, b.sdkDir, b.workspaceDir)
      if err != nil {
        return fmt.Errorf("remap.GenerateLabel(%q, %q, %q): %v", r, b.sdkDir, b.workspaceDir, err)
      }
      b.targets[r].override = remapLabel
    }
  }
  // Build the sourceSets map for searching for targets.
  for _, sourceSet := range rc.GetSourceSets() {
    if sourceSet.GetName() == "" {
      return fmt.Errorf("sourceSet %v requires name", sourceSet)
    }
    if sourceSet.GetDir() == "" {
      sourceSet.Dir = "."
    }
    if b.sourceSets[sourceSet.GetDir()] == nil {
      b.sourceSets[sourceSet.GetDir()] = make(map[string]*bazel.Label)
    }
    target, err := b.absLabel(sourceSet.GetDir(), sourceSet.GetName())
    if err != nil {
      return err
    }
    for _, src := range sourceSet.GetSrcs() {
      b.sourceSets[sourceSet.GetDir()][src] = target
    }
    for _, hdr := range sourceSet.GetHdrs() {
      b.sourceSets[sourceSet.GetDir()][hdr] = target
    }
  }
  if b.verbose {
    log.Printf("Using .bazerlifyrc:\n%+v", b.rc)
  }
  return nil
}

// Generate an absolute label
func (b *buildGen) absLabel(dirFromSDK, name string) (*bazel.Label, error) {
  absDir := filepath.Join(b.sdkDir, dirFromSDK)
  return bazel.NewLabel(absDir, name, b.workspaceDir)
}

func (b *buildGen) checkResolvable() error {
  unresolved := make(map[string]*possibleTargets) // maps name -> possible targets
  for name, possibleTargets := range b.targets {
    // If there's a target override, then this is resolved.
    if possibleTargets.override != nil {
      continue
    }
    // If there's only 1 possible target, then this can be resolved.
    if len(possibleTargets.possible) == 1 {
      continue
    }
    // Targets that aren't required by anything can happen if they've all been
    // resolved by relative lookups.
    if len(possibleTargets.requiredBy) == 0 {
      continue
    }
    // If none of these cases match, then we're unresolved.
    unresolved[name] = possibleTargets
  }
  if len(unresolved) > 0 {
    return errors.New(b.generateResolutionHint(unresolved))
  }
  return nil
}

func (b *buildGen) outputFiles() error {
  files := make(map[string]*buildfile.File) // target directory -> BUILD file

  // Generate remap rules in the BUILD file for the SDK directory
  sdkFromWorkspace, err := filepath.Rel(b.workspaceDir, b.sdkDir)
  if err != nil {
    return err
  }
  files[b.sdkDir] = buildfile.New(b.sdkDir)
  files[b.sdkDir].AddLoad(&buildfile.Load{
    Source: "@rules_cc//cc:defs.bzl",
    Symbols: []string{"cc_library"},
  })
  r := remap.New(b.rc.GetRemaps(), sdkFromWorkspace)
  for _, lib := range r.Libraries() {
    files[b.sdkDir].AddLibrary(lib)
  }
  for _, labelSetting := range r.LabelSettings() {
    files[b.sdkDir].AddLabelSetting(labelSetting)
  }
  if err := ioutil.WriteFile(filepath.Join(b.sdkDir, bzlFilename), r.BzlContents(), 0644); err != nil {
    return err
  }

  // Loop through each target, and add the library to the given file
  for _, config := range b.targets {
    for _, target := range config.possible {
      if b.searchSourceSets(filepath.Join(b.workspaceDir, target.label.Dir()), target.hdrs[0]) != nil {
        continue
      }
      b.addLibrary(target, files)
    }
  }

  // Loop through the source set targets, which have already been resolved.
  for _, sourceSet := range b.sourceSetTargets {
    b.addLibrary(sourceSet.target, files)
  }

  // Write all BUILD files to disk.
  for _, file := range files {
    if err := file.Write(); err != nil {
      return err
    }
  }

  return nil
}

func (b *buildGen) addLibrary(target *targetInfo, files map[string]*buildfile.File) {
  var deps []string
  for _, include := range target.includes {
    depLabel, err := b.resolveIncludeLabel(include)
    if err != nil {
      deps = append(deps, err.Error())
    } else {
      deps = append(deps, depLabel.RelativeTo(target.label))
    }
  }
  for _, resolved := range target.resolvedTargets {
    deps = append(deps, resolved.RelativeTo(target.label))
  }
  
  // Sort the srcs, hdrs, and deps so output has a deterministic order.
  // This is especially useful for tests.
  sort.Strings(target.srcs)
  sort.Strings(target.hdrs)
  sort.Strings(deps)

  // Find or create a new BUILD file, and add our library to it.
  if files[target.label.Dir()] == nil {
    files[target.label.Dir()] = buildfile.New(filepath.Join(b.workspaceDir, target.label.Dir()))
    files[target.label.Dir()].AddLoad(&buildfile.Load{
      Source: "@rules_cc//cc:defs.bzl",
      Symbols: []string{"cc_library"},
    })
  }

  files[target.label.Dir()].AddLibrary(&buildfile.Library{
    Name:     target.label.Name(),
    Srcs:     target.srcs,
    Hdrs:     target.hdrs,
    Deps:     deps,
    Includes: []string{"."},
  })
}

func (b *buildGen) resolveIncludeLabel(include string) (*bazel.Label, error) {
  possibleTargets := b.targets[include]
  if possibleTargets == nil {
    // This should never happen since we do pre-processing. Just crash if it occurs.
    log.Fatalf("b.targets[%s] is nil, this should never happen!", include)
  }
  if possibleTargets.override != nil {
    return possibleTargets.override, nil
  }
  if got, want := len(possibleTargets.possible), 1; got != want {
    // This should never happen since we do pre-processing. Just crash if it occurs.
    log.Fatalf("len(b.targets[%s].possible)=%d, want %d", include, got, want)
  }
  return b.resolveTargetLabel(possibleTargets)
}

// resolveTargetLabel resolves all possible targets and outputs the correct label.
// If resolution can't happen, the error contains a message to show to the user.
func (b *buildGen) resolveTargetLabel(possibleTargets *possibleTargets) (*bazel.Label, error) {
  if len(possibleTargets.possible) != 1 {
    var labels []*bazel.Label
    for _, p := range possibleTargets.possible {
      labels = append(labels, p.label)
    }
    var requiredByLabels []*bazel.Label
    for _, requiredBy := range possibleTargets.requiredBy {
      requiredByLabels = append(requiredByLabels, requiredBy)
    }
    return nil, fmt.Errorf("REQUIRED BY %s PLEASE RESOLVE: %s", bazel.JoinLabelStrings(requiredByLabels, ","), bazel.JoinLabelStrings(labels, "|"))
  }
  return possibleTargets.possible[0].label, nil
}

// buildTargetsMap walks the nrf52 SDK tree, reads all source files,
// and builds the b.targets map.
func (b *buildGen) buildTargetsMap(path string, info os.FileInfo, err error) error {
  if err != nil {
    log.Printf("%s: %v", b.prettySDKPath(path), err)
    return err
  }
  relPath, err := filepath.Rel(b.sdkDir, path)
  if err != nil {
    return err
  }

  // Check to see if path is excluded.
  for _, exclude := range b.rc.GetExcludes() {
    matched, err := filepath.Match(exclude, relPath)
    if err != nil {
      return err
    }
    if matched && info.IsDir() {
      return filepath.SkipDir
    }
    if matched {
      return nil
    }
  }

  // Walk through the dir
  if info.IsDir() {
    return nil
  }

  // Remove all BUILD files
  if info.Name() == "BUILD" {
    if err := os.Remove(path); err != nil {
      return fmt.Errorf("os.Remove(%s): %v", b.prettySDKPath(path), err)
    }
  }

  // We only want to deal with .h files
  if filepath.Ext(path) != ".h" {
    return nil
  }

  // If the file is in a source set, we delay processing to a later step.
  if b.searchSourceSets(filepath.Dir(path), info.Name()) != nil {
    return nil
  }

  shortName := strings.TrimSuffix(info.Name(), ".h")
  label, err := bazel.NewLabel(filepath.Dir(path), shortName, b.workspaceDir)
  if err != nil {
    return fmt.Errorf("bazel.NewLabel(%q, %q, %q): %v", filepath.Dir(path), shortName, b.workspaceDir, err)
  }

  hIncludes, err := b.readIncludes(path, info.Name())
  if err != nil {
    return fmt.Errorf("readIncludes(%s): %v", b.prettySDKPath(path), err)
  }

  resolved, unresolved := b.splitResolvableIncludes(label, hIncludes)

  b.populateIncludesInTargets(label, unresolved)

  target := &targetInfo{
    label: label,
    hdrs:            []string{info.Name()},
    includes:        unresolved,
    resolvedTargets: resolved,
  }

  defer func() {
    if b.targets[info.Name()] == nil {
      b.targets[info.Name()] = &possibleTargets{
        requiredBy: make(map[string]*bazel.Label),
      }
    }
    b.targets[info.Name()].possible = append(b.targets[info.Name()].possible, target)
  }()

  // TODO: expand to different types of implementation files.
  cFileName := fmt.Sprintf("%s.c", shortName)
  cFilePath := filepath.Join(label.Dir(), cFileName)
  cIncludes, err := b.readIncludes(cFilePath, info.Name())
  if err != nil {
    if b.verbose {
     log.Printf("readIncludes(%s): %v", b.prettySDKPath(cFilePath), err)
    }
    return nil
  }
  cResolved, cUnresolved := b.splitResolvableIncludes(label, cIncludes)
  b.populateIncludesInTargets(label, cUnresolved)

  target.includes = append(target.includes, cUnresolved...)
  target.resolvedTargets = append(target.resolvedTargets, cResolved...)
  target.srcs = []string{cFileName}
  return nil
}

type sourceSetTarget struct {
  // The directory to add the source set to, as an absolute path.
  dir string
  // The name of the library generated by this source set.
  name string
  // The target info for this source set.
  target *targetInfo
}

func (b *buildGen) walkSourceSets() error {
  for _, sourceSet := range b.rc.GetSourceSets() {
    ownLabel, err := b.absLabel(sourceSet.GetDir(), sourceSet.GetName())
    if err != nil {
      return err
    }

    var files []string
    for _, src := range sourceSet.GetSrcs() {
      files = append(files, filepath.Join(b.sdkDir, sourceSet.GetDir(), src))
    }
    for _, hdr := range sourceSet.GetHdrs() {
      files = append(files, filepath.Join(b.sdkDir, sourceSet.GetDir(), hdr))
    }
    uniqueResolved := make(map[string]*bazel.Label)
    uniqueUnresolved := make(map[string]bool)
    for _, file := range files {
      includes, err := b.readIncludes(file, "")
      if err != nil {
        return fmt.Errorf("readIncludes(%s): %v", file, err)
      }
      resolved, unresolved := b.splitResolvableIncludes(ownLabel, includes)
      for _, r := range resolved {
        if r == ownLabel {
          continue
        }
        uniqueResolved[r.String()] = r
      }
      for _, u := range unresolved {
        uniqueUnresolved[u] = true
      }
      b.populateIncludesInTargets(ownLabel, unresolved)
    }
    buildFileDir := filepath.Join(b.sdkDir, sourceSet.GetDir())
    var resolved []*bazel.Label
    for _, label := range uniqueResolved {
      resolved = append(resolved, label)
    }
    var unresolved []string
    for u := range uniqueUnresolved {
      unresolved = append(unresolved, u)
    }
    label, err := bazel.NewLabel(buildFileDir, sourceSet.GetName(), b.workspaceDir)
    if err != nil {
      return fmt.Errorf("bazel.NewLabel(%q, %q, %q): %v", buildFileDir, sourceSet.GetName(), b.workspaceDir, err)
    }
    b.sourceSetTargets = append(b.sourceSetTargets, &sourceSetTarget{
      dir: buildFileDir,
      name: sourceSet.GetName(),
      target: &targetInfo{
        label: label,
        hdrs: sourceSet.GetHdrs(),
        srcs: sourceSet.GetSrcs(),
        includes: unresolved,
        resolvedTargets: resolved,
      },
    })
  }
  return nil
}

// Split includes that can be resolved early.
// The remaining includes need to go through the dynamic resolution phase.
func (b *buildGen) splitResolvableIncludes(ownLabel *bazel.Label, includes []string) (resolved []*bazel.Label, unresolved []string) {
  for _, include := range includes {
    // Start by looking for overridden targets
    if target := b.targets[include]; target != nil {
      if target.override != nil {
        resolved = append(resolved, target.override)
        continue
      }
    }

    // Perform a search for the file through the include_dirs in bazelifyrc,
    // and the current library's directory.
    searchPaths := make([]string, 0, len(b.rc.GetIncludeDirs()) + 1)
    searchPaths = append(searchPaths, filepath.Join(b.workspaceDir, ownLabel.Dir()))
    // Make all search paths absolute. They are relative to the SDK directory,
    // so append it to the SDK directory, and make it absolute and cleaned.
    for _, includeDir := range b.rc.GetIncludeDirs() {
      joined := filepath.Join(b.sdkDir, includeDir)
      abs, err := filepath.Abs(joined)
      if err != nil {
        log.Printf("filepath.Abs(%s): %v", joined, err)
        continue
      }
      searchPaths = append(searchPaths, abs)
    }

    // Stat all instances of the include. If we find a relative include that matches,
    // format the target and resolve it.
    foundRelative := false
    for _, searchPath := range searchPaths {
      search := filepath.Clean(filepath.Join(searchPath, include))
      info, err := os.Stat(search)
      if err != nil {
        continue
      }
      if info.IsDir() {
        continue
      }
      foundRelative = true
      searchDir := filepath.Dir(search)
      target := b.searchSourceSets(searchPath, info.Name())
      if target == nil {
        shortName := strings.TrimSuffix(info.Name(), filepath.Ext(info.Name()))
        label, err := bazel.NewLabel(searchDir, shortName, b.workspaceDir)
        if err != nil {
          log.Fatalf("bazel.NewLabel(%q, %q, %q): %v", searchDir, shortName, b.workspaceDir, err)
        }
        target = label
      }
      // Remove dependencies on ourselves.
      if target.String() == ownLabel.String() {
        continue
      }
      resolved = append(resolved, target)
      break
    }
    if foundRelative {
      continue
    }

    unresolved = append(unresolved, include)
  }
  return
}

// We need to make sure b.targets contains an entry for each include that we need.
// When we go through with resolveTargets, we'll check that every target has
// exactly 1 possible target. If unpopulated, these will show up as 0 possible targets
func (b *buildGen) populateIncludesInTargets(requiredBy *bazel.Label, includes []string) {
  for _, include := range includes {
    if b.targets[include] == nil {
      b.targets[include] = &possibleTargets{
        requiredBy: make(map[string]*bazel.Label),
      }
    }
    b.targets[include].requiredBy[requiredBy.String()] = requiredBy
  }
}

func (b *buildGen) prettySDKPath(path string) string {
  return "<SDK>" + strings.TrimPrefix(path, b.sdkDir)
}

// readIncludes reads all the #include lines from
func (b *buildGen) readIncludes(path string, exclude string) ([]string, error) {
  file, err := os.Open(path)
  if err != nil {
    return nil, err
  }
  defer file.Close()

  scanner := bufio.NewScanner(file)
  var out []string
  for scanner.Scan() {
    line := scanner.Text()
    matches := includeMatcher.FindStringSubmatch(line)
    if len(matches) != 2 {
      if matches != nil {
        log.Printf("Reading includes from %s: len(%v) != 2", b.prettySDKPath(path), matches)
      }
      continue
    }
    if matches[1] == exclude {
      continue
    }
    if b.shouldIgnore(matches[1]) {
      continue
    }
    out = append(out, matches[1])
  }
  return out, nil
}

func (b *buildGen) shouldIgnore(header string) bool {
  for _, ignore := range b.rc.GetIgnoreHeaders() {
    if header == ignore {
      return true
    }
  }
  return false
}

// searchSourceSets searches through our source sets for the given header files
// and generates the correct target if found.
// If not found, we return an empty string.
func (b *buildGen) searchSourceSets(dir, base string) *bazel.Label {
  // We want it relative to the SDK directory, since source sets use this.
  relDir, err := filepath.Rel(b.sdkDir, dir)
  if err != nil {
    // If we write this correctly, this should never happen. Crash.
    log.Fatalf("filepath.Rel(%q, %q): %v", b.sdkDir, dir, err)
  }
  if b.sourceSets[relDir] == nil {
    return nil
  }
  return b.sourceSets[relDir][base]
}

type targetInfo struct {
  label *bazel.Label
  hdrs, srcs []string
  // These are includes that aren't relative to this target's base directory.
  // These will be resolved to targets in resolveTargets.
  includes   []string
  // A list of targets that we already resolved.
  // This happens when we find a relative path include
  // which does not need to go through the target resolution process.
  resolvedTargets []*bazel.Label
}

type possibleTargets struct {
  override *bazel.Label
  possible []*targetInfo
  // A list of targets that require this dynamic target.
  // Only for the dynamic resolution phase.
  requiredBy map[string]*bazel.Label
}

func (b *buildGen) generateResolutionHint(unresolved map[string]*possibleTargets) string {
  rc := proto.Clone(b.rc).(*bazelifyrc.Configuration)
  if rc == nil {
    rc = &bazelifyrc.Configuration{}
  }
  if rc.GetTargetOverrides() == nil {
    rc.TargetOverrides = make(map[string]string)
  }
  for name, possible := range unresolved {
    if override := possible.override; override != nil {
      log.Fatalf("No resolution hint needed for include %q with override %q", name, override)
    }
    if override := rc.GetTargetOverrides()[name]; override != "" {
      log.Fatalf("Override already exists for include %q: %q", name, override)
    }
    label, err := b.resolveTargetLabel(possible)
    if err != nil {
      rc.GetTargetOverrides()[name] = err.Error()
    } else {
      rc.GetTargetOverrides()[name] = label.String()
    }
  }
  rcText := proto.MarshalTextString(rc)
  rcPath := filepath.Join(b.sdkDir, rcFilename)
  rcHintPath := rcPath + ".hint"
  verboseText := ""
  if b.verbose {
    verboseText = fmt.Sprintf("\n.bazelifyrc.hint contents:\n%s", rcText)
  }
  if err := ioutil.WriteFile(rcHintPath, []byte(rcText), 0640); err != nil {
    return fmt.Sprintf("Found unresolved targets. Failed to write hint file: %v%s", err, verboseText)
  }
  return fmt.Sprintf("Found unresolved targets. Please add the resolutions to %s and try again. Hint written to %s%s", rcPath, rcHintPath, verboseText)
}

func (b *buildGen) removeStaleHint() error {
  hintFile := filepath.Join(b.sdkDir, ".bazelifyrc.hint")
  if _, err := os.Stat(hintFile); err != nil {
    return nil
  }
  return os.Remove(hintFile)
}
