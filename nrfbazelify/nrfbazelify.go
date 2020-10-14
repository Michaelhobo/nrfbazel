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

	"github.com/Michaelhobo/nrfbazel/internal/buildfile"
	"github.com/golang/protobuf/proto"

	"github.com/Michaelhobo/nrfbazel/bazelifyrc"
)

const (
	// We read this file from the root of the SDK.
	rcFilename = ".bazelifyrc"
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
	}
	return gen.generate()
}

// buildGen generates Bazel BUILD files for an Nrf52 SDK.
type buildGen struct {
	// These are pre-cleaned by GenerateBuildFiles
	workspaceDir, sdkDir string
	verbose bool
	targets              map[string]*possibleTargets // include name -> all possible targets
	rc                   *bazelifyrc.Configuration
}

func (b *buildGen) generate() error {
	if err := b.loadBazelifyRC(); err != nil {
		return fmt.Errorf("not loading .bazelifyrc: %v", err)
	}
	if err := filepath.Walk(b.sdkDir, b.buildTargetsMap); err != nil {
		return fmt.Errorf("filepath.Walk(%s): %v", b.sdkDir, err)
	}
	if err := b.resolveTargets(); err != nil {
		return fmt.Errorf("failed to resolve targets: %v", err)
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
		return fmt.Errorf("Could not read %s: %v", rcFilename, err)
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
		b.targets[name] = &possibleTargets{
			override: override,
		}
	}
	if b.verbose {
		log.Printf("Using .bazerlifyrc:\n%+v", b.rc)
	}
	return nil
}

func (b *buildGen) resolveTargets() error {
	unresolved := make(map[string]*possibleTargets) // maps name -> possible targets
	for name, possibleTargets := range b.targets {
		if possibleTargets.override == "" && len(possibleTargets.possible) != 1 {
			unresolved[name] = possibleTargets
		}
	}
	if len(unresolved) > 0 {
		return errors.New(b.generateResolutionHint(unresolved))
	}
	// Loop through each target, and call buildfile.WriteLibrary()
	for _, config := range b.targets {
		for _, target := range config.possible {
			var deps []string
			for _, include := range target.includes {
				deps = append(deps, b.targetFromInclude(include, target.dir))
			}
			for _, resolved := range target.resolvedTargets {
				deps = append(deps, resolved)
			}
			
			// Sort the srcs, hdrs, and deps so output has a deterministic order.
			// This is especially useful for tests.
			sort.Strings(target.srcs)
			sort.Strings(target.hdrs)
			sort.Strings(deps)

			if err := buildfile.WriteLibrary(&buildfile.Library{
				Dir:      target.dir,
				Name:     strings.TrimSuffix(target.hdrs[0], ".h"),
				Srcs:     target.srcs,
				Hdrs:     target.hdrs,
				Deps:     deps,
				Includes: []string{"."},
			}); err != nil {
				return err
			}
		}
	}
	return nil
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
	shortName := strings.TrimSuffix(info.Name(), ".h")
	dirName := filepath.Dir(path)

	hIncludes, err := b.readIncludes(path, info.Name())
	if err != nil {
		log.Printf("readIncludes(%s): %v", b.prettySDKPath(path), err)
		return nil
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
			resolved = append(resolved, b.formatTarget(&possibleTargets{
				possible: []*targetInfo{
					{
						dir: filepath.Dir(search),
						hdrs: []string{filepath.Base(search)},
					},
				},
			}, dir))
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
	// A list of targets that require this dynamic target. For debugging only.
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
