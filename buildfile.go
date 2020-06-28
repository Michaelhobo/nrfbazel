package buildfile

import (
	"fmt"
	"log"
	"path/filepath"
)

const (
)

type LibraryInfo struct {
	// directory where the BUILD file is written
	Dir string
	// name of the library rule
	Name string
	Srcs []string
	Hdrs []string
	Deps []string
	Includes []string
}

func WriteLibrary(lib *LibraryInfo) error {
	out := fmt.Sprintf("cc_library(name=%q", lib.Name)
	if lib.Srcs != nil {
		out += fmt.Sprintf(", srcs = %s", bazelStringList(lib.Srcs))
	}
	if lib.Hdrs != nil {
		out += fmt.Sprintf(", hdrs = %s", bazelStringList(lib.Hdrs))
	}
	if lib.Includes != nil {
		out += fmt.Sprintf(", includes = %s", bazelStringList(lib.Includes))
	}
	if lib.Deps != nil {
		out += fmt.Sprintf(", deps = %s", bazelStringList(lib.Deps))
	}
	out += ")"
	outFile := filepath.Join(lib.Dir, "BUILD")
	log.Printf("Writing to %s:\n%s", outFile, out)
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