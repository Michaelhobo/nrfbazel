package nrfbazelify

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Michaelhobo/nrfbazel/buildfile"
)

var testDataDir = "testdata/nrfbazelify"
const garbageText = "garbage to remove"

func mustMakeAbs(t *testing.T, dir string) string {
	t.Helper()
	abs, err := filepath.Abs(dir)
	if err != nil {
		t.Fatalf("filepath.Abs(%s): %v", dir, err)
	}
	return abs
}

func removeAllBuildFiles(t *testing.T, dir string) {
	t.Helper()
	if err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.Name() == "BUILD" {
			if err := os.Remove(path); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatalf("removeAllBuildFiles(%s): %v", dir, err)
	}
}

func checkBuildFiles(t *testing.T, libs ...*buildfile.Library) {
	t.Helper()
	gotContents := make(map[string]string) // path -> contents written to file
	wantContents := make(map[string][]string) // path -> desired contents substrings

	// Generate the content we want and the content we expect from the list of libraries.
	for _, lib := range libs {
		path, contents := buildfile.GenerateLibrary(lib)
		gotContents[path] = "" // Don't populate it yet, because we might have multiple rules in the same BUILD file.
		wantContents[path] = append(wantContents[path], contents)
	}

	// Populate allContents with contents of BUILD files
	for path := range gotContents {
		contents, err := ioutil.ReadFile(path)
		if err != nil {
			t.Errorf("Failed to read file %s: %v", path, err)
			continue
		}
		gotContents[path] = string(contents)
	}

	// Compare
	for path, wantContentsList := range wantContents {
		for _, wantContents := range wantContentsList {
			if got, want := strings.Count(gotContents[path], wantContents), 1; got != want {
				t.Errorf("%s contains %d copies of the content, want %d\n  got (full file):  %s\n  want (substring): %s", path, got, want, gotContents[path], wantContents)
			}
		}
	}
}

func TestGenerateBuildFiles_Nominal(t *testing.T) {
	workspaceDir := mustMakeAbs(t, testDataDir)
	sdkDir := filepath.Join(workspaceDir, "nominal")
	t.Cleanup(func() {
		removeAllBuildFiles(t, sdkDir)
	})
	if err := GenerateBuildFiles(workspaceDir, sdkDir); err != nil {
		t.Fatalf("GenerateBuildFiles(%s, %s): %v", testDataDir, sdkDir, err)
	}
	checkBuildFiles(t, &buildfile.Library{
		Dir: sdkDir,
		Name: "a",
		Hdrs: []string{"a.h"},
		Deps: []string{":b"},
		Includes: []string{"."},
	},
	&buildfile.Library{
		Dir: sdkDir,
		Name: "b",
		Srcs: []string{"b.c"},
		Hdrs: []string{"b.h"},
		Deps: []string{"//nominal/dir:c"},
		Includes: []string{"."},
	},
	&buildfile.Library{
		Dir: filepath.Join(sdkDir, "dir"),
		Name: "c",
		Srcs: []string{"c.c"},
		Hdrs: []string{"c.h"},
		Includes: []string{"."},
	})
}

func TestGenerateBuildFiles_NameMatchesDir(t *testing.T) {
	workspaceDir := mustMakeAbs(t, testDataDir)
	sdkDir := filepath.Join(workspaceDir, "name_matches_dir")
	t.Cleanup(func() {
		removeAllBuildFiles(t, sdkDir)
	})
	if err := GenerateBuildFiles(workspaceDir, sdkDir); err != nil {
		t.Fatalf("GenerateBuildFiles(%s, %s): %v", testDataDir, sdkDir, err)
	}
	checkBuildFiles(t, &buildfile.Library{
		Dir: sdkDir,
		Name: "uses_dir",
		Hdrs: []string{"uses_dir.h"},
		Deps: []string{"//name_matches_dir/dir"},
		Includes: []string{"."},
	},
	&buildfile.Library{
		Dir: filepath.Join(sdkDir, "dir"),
		Name: "dir",
		Hdrs: []string{"dir.h"},
		Includes: []string{"."},
	})
}

func TestGenerateBuildFiles_BuildFileExists(t *testing.T) {
	workspaceDir := mustMakeAbs(t, testDataDir)
	sdkDir := filepath.Join(workspaceDir, "build_file_exists")
	garbageBuild := filepath.Join(sdkDir, "BUILD")
	if err := ioutil.WriteFile(garbageBuild, []byte(garbageText), 0644); err != nil {
		t.Fatalf("ioutil.WriteFile(%s, %s): %v", garbageBuild, garbageText, err)
	}
	t.Cleanup(func() {
		removeAllBuildFiles(t, sdkDir)
	})
	if err := GenerateBuildFiles(workspaceDir, sdkDir); err != nil {
		t.Fatalf("GenerateBuildFiles(%s, %s): %v", testDataDir, sdkDir, err)
	}
	checkBuildFiles(t, &buildfile.Library{
		Dir: sdkDir,
		Name: "a",
		Hdrs: []string{"a.h"},
		Includes: []string{"."},
	})
	buildPath := filepath.Join(sdkDir, "BUILD")
	contents, err := ioutil.ReadFile(buildPath)
	if err != nil {
		t.Fatalf("ioutil.ReadFile(%s): %v", buildPath, err)
	}
	if got, dontWant := string(contents), "garbage to remove"; strings.Contains(got, dontWant) {
		t.Errorf("strings.Contains(%s, %s): got true, want false", got, dontWant)
	}
}

func TestGenerateBuildFiles_WorkspaceMatchesSDKDir(t *testing.T) {
	workspaceAndSDKDir := filepath.Join(mustMakeAbs(t, testDataDir), "workspace_matches_sdk_dir")
	t.Cleanup(func() {
		removeAllBuildFiles(t, workspaceAndSDKDir)
	})
	if err := GenerateBuildFiles(workspaceAndSDKDir, workspaceAndSDKDir); err != nil {
		t.Fatalf("GenerateBuildFiles(%s, %s): %v", testDataDir, workspaceAndSDKDir, err)
	}
	checkBuildFiles(t, &buildfile.Library{
		Dir: workspaceAndSDKDir,
		Name: "a",
		Hdrs: []string{"a.h"},
		Deps: []string{":workspace_matches_sdk_dir"},
		Includes: []string{"."},
	},
	&buildfile.Library{
		Dir: workspaceAndSDKDir,
		Name: "workspace_matches_sdk_dir",
		Srcs: []string{"workspace_matches_sdk_dir.c"},
		Hdrs: []string{"workspace_matches_sdk_dir.h"},
		Includes: []string{"."},
	})
}


func TestGenerateBuildFiles_IncludeDoesNotExist(t *testing.T) {
	workspaceDir := mustMakeAbs(t, testDataDir)
	sdkDir := filepath.Join(workspaceDir, "include_does_not_exist")
	if err := GenerateBuildFiles(workspaceDir, sdkDir); err == nil {
		t.Errorf("GenerateBuildFiles(%s, %s): got nil error, want an error", workspaceDir, sdkDir)
	}
}