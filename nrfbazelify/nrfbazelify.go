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
func GenerateBuildFiles(workspaceDir, sdkDir string) error {
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
		sdkDir:       filepath.Clean(sdkDir),
		targets:      make(map[string]*possibleTargets),
	}
	return gen.generate()
}

// BuildGenerator generates Bazel BUILD files for an Nrf52 SDK.
type buildGen struct {
	// These are pre-cleaned by GenerateBuildFiles
	workspaceDir, sdkDir string
	targets              map[string]*possibleTargets // include name -> all possible targets
	rc                   *bazelifyrc.Configuration
}

func (b *buildGen) generate() error {
	if err := b.loadBazelifyRC(); err != nil {
		log.Printf("Not loading .bazelifyrc: %v", err)
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
		log.Fatalf("b.targets[%s] is nil", include)
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

// formatInclude formats the Bazel target.
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
	return fmt.Sprintf("PLEASE RESOLVE: %s", strings.Join(formatted, "|"))
}

// buildTargetsMap walks the nrf52 SDK tree, reads all source files,
// and builds the b.targets map.
func (b *buildGen) buildTargetsMap(path string, info os.FileInfo, err error) error {
	if err != nil {
		log.Printf("%s: %v", b.prettySDKPath(path), err)
		return nil
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
	b.populateIncludesInTargets(hIncludes)

	target := &targetInfo{
		dir:      dirName,
		hdrs:     []string{info.Name()},
		includes: hIncludes,
	}

	defer func() {
		if b.targets[info.Name()] == nil {
			b.targets[info.Name()] = &possibleTargets{}
		}
		b.targets[info.Name()].possible = append(b.targets[info.Name()].possible, target)
	}()

	// TODO: expand to different types of implementation files.
	cFileName := fmt.Sprintf("%s.c", shortName)
	cIncludes, err := b.readIncludes(filepath.Join(dirName, cFileName), info.Name())
	if err != nil {
		// TODO: hide this one behind a verbose flag
		log.Printf("readIncludes(%s): %v", b.prettySDKPath(path), err)
		return nil
	}
	b.populateIncludesInTargets(cIncludes)
	target.includes = append(target.includes, cIncludes...)
	target.srcs = []string{cFileName}
	return nil
}

// We need to make sure b.targets contains an entry for each include that we need.
// When we go through with resolveTargets, we'll check that every target has
// exactly 1 possible target. If unpopulated, these will show up as 0 possible targets
func (b *buildGen) populateIncludesInTargets(includes []string) {
	for _, include := range includes {
		if b.targets[include] == nil {
			b.targets[include] = &possibleTargets{}
		}
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
		out = append(out, matches[1])
	}
	return out, nil
}

type targetInfo struct {
	dir        string
	hdrs, srcs []string
	includes   []string
}

type possibleTargets struct {
	override string
	possible []*targetInfo
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
		rc.GetTargetOverrides()[name] = b.formatTarget(possible, "")
	}
	rcText := proto.MarshalTextString(rc)
	rcPath := filepath.Join(b.sdkDir, rcFilename)
	return fmt.Sprintf("Found unresolved targets. Please add the resolutions to %s and try again:\n\n%s", rcPath, rcText)
}
