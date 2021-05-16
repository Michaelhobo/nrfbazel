// Package remap handles generating the contents of the remap features.
package remap

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/Michaelhobo/nrfbazel/internal/buildfile"
)

const (
  emptyRemap = "nrfbazelify_empty_remap"

)

var (
	remapBzlContents = template.Must(template.New("remapBzlContents").Parse(`
""" This allows performing remapping of library dependencies based on the
nrf_cc_binary that includes the library.
"""
load("@rules_cc//cc:defs.bzl", "cc_binary")

def _remap_transition_impl(settings, attr):
  return {
{{range .Data}}
		"{{.Label}}": attr.{{.ShortName}},
{{end}}
  }

_remap_transition = transition(
  implementation = _remap_transition_impl,
  inputs = [],
  outputs = [
{{range .Data}}
    "{{.Label}}",
{{end}}
  ],
)

# All this does is copy the cc_binary's output to its own output and propagate
# its runfiles and executable so "bazel run" works.
def _remap_rule_impl(ctx):
  actual_binary = ctx.attr.actual_binary[0]
  outfile = ctx.actions.declare_file(ctx.label.name)
  cc_binary_outfile = actual_binary[DefaultInfo].files.to_list()[0]

  ctx.actions.run_shell(
    inputs = [cc_binary_outfile],
    outputs = [outfile],
    command = "cp {} {}".format(cc_binary_outfile.path, outfile.path),
  )
  return [
    DefaultInfo(
      executable = outfile,
      data_runfiles = actual_binary[DefaultInfo].data_runfiles,
    ),
  ]

# Enable us to remap certain files dynamically.
_remap_rule = rule(
  implementation = _remap_rule_impl,
  attrs = {
{{range .Data}}
    "{{.ShortName}}": attr.label(),
{{end}}
    "actual_binary": attr.label(cfg = _remap_transition),
    "_whitelist_function_transition": attr.label(
      default = "@bazel_tools//tools/whitelists/function_transition_whitelist",
    ),
  },
  # Making this executable means it works with "$ bazel run".
  executable = True,
)

# Convenience macro: this instantiates a transition_rule with the given
# desired features, instantiates a cc_binary as a dependency of that rule,
# and fills out the cc_binary with all other parameters passed to this macro.
def nrf_cc_binary(name, remap = None, **kwargs):
  """A cc_binary with configurable targets.

  Args:
    name: string name of the binary.
    remap: dict of target names to rules.
    **kwargs: args passed to the underlying cc_binary rule
  """
  cc_binary_name = name + "_native_binary"
  _remap_rule(
    name = name,
    actual_binary = ":{}".format(cc_binary_name),
{{range .Data}}
		{{.ShortName}} = remap.get("{{.Header}}", "{{.BuildSettingDefault}}"),
{{end}}
  )
  cc_binary(
    name = cc_binary_name,
    **kwargs
  )
`))
)

// New creates a new remap from a list of header files from
// bazelifyrc.Configuration's remaps field.
// sdkFromWorkspace is the relative path from sdkDir to workspaceDir.
func New(headers []string, sdkFromWorkspace string) (*Remaps, error) {
  var libs []*buildfile.Library
  if len(headers) != 0 {
    libs = append(libs, &buildfile.Library{Name: emptyRemap})
  }
  labelSettings := make(map[string]*buildfile.LabelSetting)
	remaps := &RemapsData{}
  for _, header := range headers {
    if labelSettings[header] != nil {
      return nil, fmt.Errorf("duplicate remap for header file %q", header)
    }

    shortName := strings.TrimSuffix(header, filepath.Ext(header))
    remapName := fmt.Sprintf("%s_remap", shortName)
    buildSettingDefault := fmt.Sprintf("//%s:%s", sdkFromWorkspace, emptyRemap)
    labelSettings[header] = &buildfile.LabelSetting{
      Name: remapName,
      BuildSettingDefault: buildSettingDefault,
    }
    label := fmt.Sprintf("//%s", sdkFromWorkspace)
    if filepath.Base(sdkFromWorkspace) != remapName {
      label += fmt.Sprintf(":%s", remapName)
    }
    remaps.Data = append(remaps.Data, &Processed{
      Header: header,
      ShortName: shortName,
      Label: label,
      BuildSettingDefault: buildSettingDefault,
    })
  }
	var bzlContents bytes.Buffer
  if err := remapBzlContents.Execute(&bzlContents, remaps); err != nil {
		return nil, fmt.Errorf("template execution failed: %v", err)
	}

  return &Remaps{
    libs: libs,
    labelSettings: labelSettings,
    bzlContents: bzlContents.Bytes(),
  }, nil
}

type RemapsData struct {
	Data []*Processed
}

type Processed struct {
  // The original header name
  Header string
  // The name of the file without the extension
  ShortName string
  // The name of the remap label, which is //sdk_dir:short_name_remap
  Label string
  // If no default is provided, the build setting default label to use.
  BuildSettingDefault string
}

// Remaps holds data for remapping header files dynamically.
type Remaps struct {
  libs []*buildfile.Library
  labelSettings map[string]*buildfile.LabelSetting // header file -> label setting
  bzlContents []byte
}

// Libraries returns the libraries that need to be created.
func (r *Remaps) Libraries() []*buildfile.Library {
  return r.libs
}

// LabelSettings returns the label_attr rules that need to be created for each header file.
func (r *Remaps) LabelSettings() map[string]*buildfile.LabelSetting {
  return r.labelSettings
}

// BzlContents returns the .bzl file's contents.
func (r *Remaps) BzlContents() []byte {
  return r.bzlContents
}
