package buildfile

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
)

// New creates a new File.
func New(dir string) *File {
  if !filepath.IsAbs(dir) {
    log.Fatalf("buildfile.New(%q): absolute path required", dir)
  }
  return &File{
    Path: filepath.Join(dir, "BUILD"),
    packageVisibility: "//visibility:public",
  }
}

// File holds information for generating a BUILD file.
type File struct {
  Path string
  loads []*Load
  libs []*Library
  labelSettings []*LabelSetting
  packageVisibility string
}

// Write writes the file's generated contents to a file.
func (f *File) Write() error {
  return os.WriteFile(f.Path, []byte(f.Generate()), 0644)
}

// Generate generates the output contents of the file.
func (f *File) Generate() string {
  var out string

  // Generate load statements
  sort.Slice(f.loads, func(i, j int) bool{
    return f.loads[i].Source < f.loads[j].Source
  })
  for _, load := range f.loads {
    out += load.Generate() + "\n"
  }

  // Add default visibility
  out += fmt.Sprintf("package(default_visibility=[%q])\n", f.packageVisibility)

  // Generate all libraries
  sort.Slice(f.libs, func(i, j int) bool {
    return f.libs[i].Name < f.libs[j].Name
  })
  for _, lib := range f.libs {
    out += lib.Generate() + "\n"
  }

  // Generate all label_settings
  sort.Slice(f.labelSettings, func(i, j int) bool {
    return f.labelSettings[i].Name < f.labelSettings[j].Name
  })
  for _, labelSetting := range f.labelSettings {
    out += labelSetting.Generate() + "\n"
  }

  return out
}

// AddLoad adds a load statement to this file.
func (f *File) AddLoad(load *Load) {
  f.loads = append(f.loads, load)
}

// AddLibrary adds a library to this file.
func (f *File) AddLibrary(lib *Library) {
  f.libs = append(f.libs, lib)
}

// AddLabelSetting adds a label_setting to this file.
func (f *File) AddLabelSetting(labelSetting *LabelSetting) {
  f.labelSettings = append(f.labelSettings, labelSetting)
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

// LabelSetting represents a label_setting rule.
type LabelSetting struct {
  Name string
  BuildSettingDefault string
}

// Generate generates the output format of this label_setting.
func (l *LabelSetting) Generate() string {
  return fmt.Sprintf("label_setting(name=%q, build_setting_default=%q)", l.Name, l.BuildSettingDefault)
}

// Load represents a load() statement.
type Load struct {
  Source string
  Symbols []string
}

// Generate generates the output format of this load statement.
func (l *Load) Generate() string {
  contents := fmt.Sprintf("load(%q", l.Source)
  for _, symbol := range l.Symbols {
    contents += fmt.Sprintf(", %q", symbol)
  }
  contents += ")"
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
