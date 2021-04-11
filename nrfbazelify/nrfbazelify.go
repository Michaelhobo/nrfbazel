// Package nrfbazelify converts nRF5 SDKs to use Bazel.
// This contains a Bazel BUILD file generator that reads source files
// and does the heavy lifting of resolving targets and writing BUILD files.
package nrfbazelify

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/Michaelhobo/nrfbazel/internal/buildfile"
	"github.com/Michaelhobo/nrfbazel/internal/remap"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"

	"github.com/Michaelhobo/nrfbazel/proto/bazelifyrc"
)

const (
  // We write the contents of our remap features to this file.
  bzlFilename = "remap.bzl"
)

var (
  // dotGraphPath is the path to write the DOT graph for debug and visualization.
  dotGraphPath = flag.String("dot_graph_path", "", "The path to write the DOT graph. Omit to skip DOT graph output.")
  includeMatcher = regexp.MustCompile("^\\s*#include\\s+\"(.+)\".*$")
)
// GenerateBuildFiles generates BUILD files. Use this to switch between V1 and V2.
var GenerateBuildFiles = GenerateBuildFilesV1

// GenerateBuildFilesV2 generates BUILD files with a new algorithm that removes cyclic dependencies.
func GenerateBuildFilesV2(workspaceDir, sdkDir string, verbose bool) error {
  if !filepath.IsAbs(workspaceDir) {
    return errors.New("workspace must be an absolute path")
  }
  if !filepath.IsAbs(sdkDir) {
    return errors.New("sdk_dir must be an absolute path")
  }
  if !strings.HasPrefix(sdkDir, workspaceDir) {
    return fmt.Errorf("sdk_dir is not inside workspace_dir:\nsdk_dir=%s\nworkspace_dir=%s", sdkDir, workspaceDir)
  }
  conf, err := ReadConfig(sdkDir, workspaceDir, verbose)
  if err != nil {
    return fmt.Errorf("ReadBazelifyRC: %v", err)
  }
  log.Printf("Generating BUILD files for %s", sdkDir)
  graph := NewDependencyGraph(sdkDir, workspaceDir)
  if *dotGraphPath != "" {
    defer func(path string) {
      log.Printf("Saving dependency graph to %s", path)
      if err := graph.OutputDOTGraph(path); err != nil {
        log.Printf("OutputDOTGraph(%q): %v", path, err)
      }
    }(*dotGraphPath)
  }
  walker, err := NewSDKWalker(conf, graph)
  if err != nil {
    return fmt.Errorf("NewSDKWalker: %v", err)
  }
  unresolvedDeps, err := walker.PopulateGraph()
  if err != nil {
    return fmt.Errorf("SDKWalker.PopulateGraph: %v", err)
  }
  if len(unresolvedDeps) > 0 {
    return WriteNewHint(conf, unresolvedDeps)
  }
  if err := OutputBuildFiles(conf, graph); err != nil {
    return fmt.Errorf("OutputBuildFiles: %v", err)
  }
  if err := RemoveStaleHint(sdkDir); err != nil {
    return fmt.Errorf("removeStaleHintFile: %v", err)
  }
  return nil
}

// GenerateBuildFilesV1 generates BUILD files for all C source files in sdkDir
// and marks all includes starting from workspaceDir.
func GenerateBuildFilesV1(workspaceDir, sdkDir string, verbose bool) error {
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
    sourceSets:   make(map[string]map[string]string),
  }
  return gen.generate()
}

// buildGen generates Bazel BUILD files for nrf SDKs.
type buildGen struct {
  // These are pre-cleaned by GenerateBuildFiles
  workspaceDir, sdkDir string
  verbose bool
  targets              map[string]*possibleTargets // include name -> all possible targets
  sourceSets           map[string]map[string]string // dir (sdk-relative) -> header name -> target label
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
  rcData, err := os.ReadFile(rcPath)
  if err != nil {
    return fmt.Errorf("could not read %s: %v", rcFilename, err)
  }
  var rc bazelifyrc.Configuration
  if err := prototext.Unmarshal(rcData, &rc); err != nil {
    return err
  }
  b.rc = &rc
  for name, override := range rc.GetTargetOverrides() {
    if b.targets[name] != nil {
      return fmt.Errorf("duplicate target override for %q in %s", name, rcFilename)
    }
    b.targets[name] = &possibleTargets{
      override: override,
    }
  }
  sdkFromWorkspace, err := filepath.Rel(b.workspaceDir, b.sdkDir)
  if err != nil {
    return err
  }
  for _, r := range rc.GetRemaps() {
    if filepath.Ext(r) != ".h" {
      return fmt.Errorf("invalid remap %q: must have .h extension", r)
    }
    if b.targets[r] == nil {
      b.targets[r] = &possibleTargets{}
    }
    if b.targets[r].override == "" {
      b.targets[r].override = remap.GenerateLabel(r, sdkFromWorkspace)
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
      b.sourceSets[sourceSet.GetDir()] = make(map[string]string)
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
func (b *buildGen) absLabel(dirFromSDK, name string) (string, error) {
  fullDir := filepath.Join(b.sdkDir, dirFromSDK)
  dirFromWorkSpace, err := filepath.Rel(b.workspaceDir, fullDir)
  if err != nil {
    return "", err
  }
  if dirFromWorkSpace == "." {
    dirFromWorkSpace = ""
  }
  target := fmt.Sprintf("//%s", dirFromWorkSpace)
  if filepath.Base(dirFromSDK) != name {
    target += fmt.Sprintf(":%s", name)
  }
  return target, nil
}

func (b *buildGen) checkResolvable() error {
  unresolved := make(map[string]*possibleTargets) // maps name -> possible targets
  for name, possibleTargets := range b.targets {
    // If there's a target override, then this is resolved.
    if possibleTargets.override != "" {
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
  r, err := remap.New(b.rc.GetRemaps(), sdkFromWorkspace)
  if err != nil {
    return fmt.Errorf("remap.New: %v", err)
  }
  for _, lib := range r.Libraries() {
    files[b.sdkDir].AddLibrary(lib)
  }
  for _, labelSetting := range r.LabelSettings() {
    files[b.sdkDir].AddLabelSetting(labelSetting)
  }
  if err := os.WriteFile(filepath.Join(b.sdkDir, bzlFilename), r.BzlContents(), 0644); err != nil {
    return err
  }

  // Loop through each target, and add the library to the given file
  for _, config := range b.targets {
    for _, target := range config.possible {
      dir := target.dir
      name := strings.TrimSuffix(target.hdrs[0], filepath.Ext(target.hdrs[0]))
      if b.searchSourceSets(dir, target.hdrs[0]) != "" {
        continue
      }
      b.addLibrary(name, dir, target, files)
    }
  }

  // Loop through the source set targets, which have already been resolved.
  for _, sourceSet := range b.sourceSetTargets {
    b.addLibrary(sourceSet.name, sourceSet.dir, sourceSet.target, files)
  }

  // Write all BUILD files to disk.
  for _, file := range files {
    if err := file.Write(); err != nil {
      return err
    }
  }

  return nil
}

func (b *buildGen) addLibrary(name, dir string, target *targetInfo, files map[string]*buildfile.File) {
  var deps []string
  for _, include := range target.includes {
    deps = append(deps, b.targetFromInclude(include, dir))
  }
  deps = append(deps, target.resolvedTargets...)
  
  // Sort the srcs, hdrs, and deps so output has a deterministic order.
  // This is especially useful for tests.
  sort.Strings(target.srcs)
  sort.Strings(target.hdrs)
  sort.Strings(deps)

  // Find or create a new BUILD file, and add our library to it.
  if file := files[target.dir]; file == nil {
    files[dir] = buildfile.New(dir)
    files[dir].AddLoad(&buildfile.Load{
      Source: "@rules_cc//cc:defs.bzl",
      Symbols: []string{"cc_library"},
    })
  }

  files[target.dir].AddLibrary(&buildfile.Library{
    Name:     name,
    Srcs:     target.srcs,
    Hdrs:     target.hdrs,
    Deps:     deps,
    Includes: []string{"."},
  })
}

func (b *buildGen) targetFromInclude(include, ownDir string) string {
  possibleTargets := b.targets[include]
  if possibleTargets == nil {
    // This should never happen since we do pre-processing. Just crash if it occurs.
    log.Fatalf("b.targets[%s] is nil, this should never happen!", include)
  }
  if possibleTargets.override != "" {
    return possibleTargets.override
  }
  if got, want := len(possibleTargets.possible), 1; got != want {
    // This should never happen since we do pre-processing. Just crash if it occurs.
    log.Fatalf("len(b.targets[%s].possible)=%d, want %d", include, got, want)
  }
  return b.formatTarget(possibleTargets, ownDir)
}

// formatTarget formats the Bazel target.
// If possibleTargets does not have an override or does not have exactly 1 possible target,
// we will print a PLEASE RESOLVE in the output.
func (b *buildGen) formatTarget(possibleTargets *possibleTargets, ownDir string) string {
  var formatted []string
  for _, possible := range possibleTargets.possible {
    prefix := ""
    suffix := ""
    if possible.dir != ownDir {
      prefix = fmt.Sprintf("/%s", strings.TrimPrefix(possible.dir, b.workspaceDir))
    }

    // If the target has a prefix of "//a/dir", and the target name is "dir",
    // we can shorten "//a/dir:dir" to just "//a/dir"
    targetName := strings.TrimSuffix(possible.hdrs[0], ".h")
    targetDirName := filepath.Base(strings.TrimPrefix(prefix, "//"))
    if targetDirName != targetName {
      suffix = fmt.Sprintf(":%s", targetName)
    }
    // Only populate the prefix with the directory if target is in a different directory.
    formatted = append(formatted, fmt.Sprintf("%s%s", prefix, suffix))
  }
  if len(formatted) == 1 {
    return formatted[0]
  }
  return fmt.Sprintf("REQUIRED BY %s PLEASE RESOLVE: %s", strings.Join(possibleTargets.requiredBy, ","), strings.Join(formatted, "|"))
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
  if b.searchSourceSets(filepath.Dir(path), info.Name()) != "" {
    return nil
  }

  shortName := strings.TrimSuffix(info.Name(), ".h")
  dirName := filepath.Dir(path)

  hIncludes, err := b.readIncludes(path, info.Name())
  if err != nil {
    return fmt.Errorf("readIncludes(%s): %v", b.prettySDKPath(path), err)
  }

  resolved, unresolved :=  b.splitResolvableIncludes(dirName, hIncludes)

  requiredBy := info.Name()
  if b.verbose {
    requiredBy = b.prettySDKPath(path)
  }
  b.populateIncludesInTargets(requiredBy, unresolved)

  target := &targetInfo{
    dir:             dirName,
    hdrs:            []string{info.Name()},
    includes:        unresolved,
    resolvedTargets: resolved,
  }

  defer func() {
    if b.targets[info.Name()] == nil {
      b.targets[info.Name()] = &possibleTargets{}
    }
    b.targets[info.Name()].possible = append(b.targets[info.Name()].possible, target)
  }()

  // TODO: expand to different types of implementation files.
  cFileName := fmt.Sprintf("%s.c", shortName)
  cFilePath := filepath.Join(dirName, cFileName)
  cIncludes, err := b.readIncludes(cFilePath, info.Name())
  if err != nil {
    if b.verbose {
     log.Printf("readIncludes(%s): %v", b.prettySDKPath(cFilePath), err)
    }
    return nil
  }
  cResolved, cUnresolved := b.splitResolvableIncludes(dirName, cIncludes)
  cRequiredBy := cFileName
  if b.verbose {
    cRequiredBy = b.prettySDKPath(cFilePath)
  }
  b.populateIncludesInTargets(cRequiredBy, cUnresolved)

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
    uniqueResolved := make(map[string]bool)
    uniqueUnresolved := make(map[string]bool)
    for _, file := range files {
      name := filepath.Base(file)
      dirName := filepath.Dir(file)
      includes, err := b.readIncludes(file, "")
      if err != nil {
        return fmt.Errorf("readIncludes(%s): %v", file, err)
      }
      resolved, unresolved := b.splitResolvableIncludes(dirName, includes)
      for _, r := range resolved {
        if r == ownLabel {
          continue
        }
        uniqueResolved[r] = true
      }
      for _, u := range unresolved {
        uniqueUnresolved[u] = true
      }
      requiredBy := name
      if b.verbose {
        requiredBy = b.prettySDKPath(file)
      }
      b.populateIncludesInTargets(requiredBy, unresolved)
    }
    buildFileDir := filepath.Join(b.sdkDir, sourceSet.GetDir())
    var resolved []string
    for r := range uniqueResolved {
      resolved = append(resolved, r)
    }
    var unresolved []string
    for u := range uniqueUnresolved {
      unresolved = append(unresolved, u)
    }
    b.sourceSetTargets = append(b.sourceSetTargets, &sourceSetTarget{
      dir: buildFileDir,
      name: sourceSet.GetName(),
      target: &targetInfo{
        dir: buildFileDir,
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
func (b *buildGen) splitResolvableIncludes(dir string, includes []string) (resolved, unresolved []string) {
  for _, include := range includes {
    // Start by looking for overridden targets
    if target := b.targets[include]; target != nil {
      if target.override != "" {
        resolved = append(resolved, target.override)
        continue
      }
    }

    // Perform a search for the file through the include_dirs in bazelifyrc,
    // and the current library's directory.
    searchPaths := make([]string, 0, len(b.rc.GetIncludeDirs()) + 1)
    searchPaths = append(searchPaths, dir)
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
      target := b.searchSourceSets(filepath.Dir(search), filepath.Base(search))
      if target == "" {
        target = b.formatTarget(&possibleTargets{
          possible: []*targetInfo{
            {
              dir: filepath.Dir(search),
              hdrs: []string{filepath.Base(search)},
            },
          },
        }, dir)
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
func (b *buildGen) populateIncludesInTargets(requiredBy string, includes []string) {
  for _, include := range includes {
    if b.targets[include] == nil {
      b.targets[include] = &possibleTargets{}
    }
    b.targets[include].requiredBy = append(b.targets[include].requiredBy, requiredBy)
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
func (b *buildGen) searchSourceSets(dir, base string) string {
  // We want it relative to the SDK directory, since source sets use this.
  relDir, err := filepath.Rel(b.sdkDir, dir)
  if err != nil {
    // If we write this correctly, this should never happen. Crash.
    log.Fatalf("filepath.Rel(%q, %q): %v", b.sdkDir, dir, err)
  }
  if b.sourceSets[relDir] == nil {
    return ""
  }
  return b.sourceSets[relDir][base]
}

type targetInfo struct {
  dir        string
  hdrs, srcs []string
  // These are includes that aren't relative to this target's base directory.
  // These will be resolved to targets in resolveTargets.
  includes   []string
  // A list of targets that we already resolved.
  // This happens when we find a relative path include
  // which does not need to go through the target resolution process.
  resolvedTargets []string
}

type possibleTargets struct {
  override string
  possible []*targetInfo
  // A list of targets that require this dynamic target.
  // Only for the dynamic resolution phase.
  requiredBy []string
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
    if override := possible.override; override != "" {
      log.Fatalf("No resolution hint needed for include %q with override %q", name, override)
    }
    if override := rc.GetTargetOverrides()[name]; override != "" {
      log.Fatalf("Override already exists for include %q: %q", name, override)
    }
    rc.GetTargetOverrides()[name] = b.formatTarget(possible, "")
  }
  rcText, err := prototext.Marshal(rc)
  if err != nil {
    log.Fatalf("prototext.Marshal resolution hint: %v", err)
  }
  rcPath := filepath.Join(b.sdkDir, rcFilename)
  rcHintPath := rcPath + ".hint"
  verboseText := ""
  if b.verbose {
    verboseText = fmt.Sprintf("\n.bazelifyrc.hint contents:\n%s", rcText)
  }
  if err := os.WriteFile(rcHintPath, []byte(rcText), 0640); err != nil {
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
