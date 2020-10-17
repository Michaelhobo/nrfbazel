package buildfile

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"sort"
)

// New creates a new File.
func New(dir string) *File {
	return &File{
		Path: filepath.Join(dir, "BUILD"),
		loads: make(map[string][]string),
		packageVisibility: "//visibility:public",
	}
}

// File holds information for generating a BUILD file.
type File struct {
	Path string
	libs []*Library
	loads map[string][]string // load file -> list of load symbols
	packageVisibility string
}

// Write writes the file's generated contents to a file.
func (f *File) Write() error {
	return ioutil.WriteFile(f.Path, []byte(f.Generate()), 0644)
}

// Generate generates the output contents of the file.
func (f *File) Generate() string {
	var out string

	// Add default visibility
	out += fmt.Sprintf("package(default_visibility=%q)\n", f.packageVisibility)

	// Generate load statements
	for file, symbols := range f.loads {
		out += fmt.Sprintf("load(%q", file)
		for _, symbol := range symbols {
			out += fmt.Sprintf(", %q", symbol)
		}
		out += ")\n"
	}

	// Sort our libs by the Name field, to provide a stable ordering.
	// This prevents churn between runs of nrfbazelify.
	sort.Slice(f.libs, func(i, j int) bool {
		return f.libs[i].Name < f.libs[j].Name
	})

	// Generate all header files
	for _, lib := range f.libs {
		out += lib.Generate() + "\n"
	}

	return out
}

// AddLibrary adds a library to this file.
func (f *File) AddLibrary(lib *Library) {
	f.libs = append(f.libs, lib)
}

// Library contains the information needed to generate a cc_library rule.
type Library struct {
	// name of the library rule
	Name     string
	Srcs     []string
	Hdrs     []string
	Deps     []string
	Includes []string
}

// Generate generates the output format of this library.
func (l *Library) Generate() string {
	contents := fmt.Sprintf("cc_library(name=%q", l.Name)
	if l.Srcs != nil {
		contents += fmt.Sprintf(", srcs = %s", bazelStringList(l.Srcs))
	}
	if l.Hdrs != nil {
		contents += fmt.Sprintf(", hdrs = %s", bazelStringList(l.Hdrs))
	}
	if l.Includes != nil {
		contents += fmt.Sprintf(", includes = %s", bazelStringList(l.Includes))
	}
	if l.Deps != nil {
		contents += fmt.Sprintf(", deps = %s", bazelStringList(l.Deps))
	}
	contents += ")\n"
	return contents

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
