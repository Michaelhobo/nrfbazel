package buildfile

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
)

// Library contains the information needed to generate a cc_library rule.
type Library struct {
	// directory where the BUILD file is written
	Dir string
	// name of the library rule
	Name     string
	Srcs     []string
	Hdrs     []string
	Deps     []string
	Includes []string
}

// GenerateLibrary generates the contents of the cc_library rule.
func GenerateLibrary(lib *Library) (path, contents string) {
	path = filepath.Join(lib.Dir, "BUILD")
	contents = fmt.Sprintf("cc_library(name=%q", lib.Name)
	if lib.Srcs != nil {
		contents += fmt.Sprintf(", srcs = %s", bazelStringList(lib.Srcs))
	}
	if lib.Hdrs != nil {
		contents += fmt.Sprintf(", hdrs = %s", bazelStringList(lib.Hdrs))
	}
	if lib.Includes != nil {
		contents += fmt.Sprintf(", includes = %s", bazelStringList(lib.Includes))
	}
	if lib.Deps != nil {
		contents += fmt.Sprintf(", deps = %s", bazelStringList(lib.Deps))
	}
	contents += ")"
	return
}

// WriteLibrary writes the contents of the cc_library rule to file.
func WriteLibrary(lib *Library) error {
	path, contents := GenerateLibrary(lib)
	log.Printf("Writing to %s:\n%s", path, contents)
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err := file.WriteString(contents); err != nil {
		return err
	}
	return nil
}

// bazelStringList converts the input slice of strings into a Bazel list
// that can be used like this: fmt.Sprintf("srcs = %s", bazelStringList(in))
func bazelStringList(in []string) string {
	first := true
	var out string
	for _, val := range in {
		if first {
			out = fmt.Sprintf("[%q", val)
			first = false
			continue
		}
		out += fmt.Sprintf(", %q", val)
	}
	out += "]"
	return out
}
