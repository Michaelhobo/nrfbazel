// Package remap handles generating the contents of the remap features.
package remap

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Michaelhobo/nrfbazel/internal/bazel"
	"github.com/Michaelhobo/nrfbazel/internal/buildfile"
)

const (
  emptyRemap = "nrfbazelify_empty_remap"

  header = `
""" This allows performing remapping of library dependencies based on the
nrf_cc_binary that includes the library.
"""
load("@rules_cc//cc:defs.bzl", "cc_binary")
`

  remapTransition = `
def _remap_transition_impl(settings, attr):
  return {
%s
  }

_remap_transition = transition(
  implementation = _remap_transition_impl,
  inputs = [],
  outputs = [
%s
  ],
)
`
  // A single line in the return statement in remapTransition.
  remapTransitionReturn = "    %q: attr.%s,\n"
  // A single line in the outputs in remapTransition.
  remapTransitionOutput = "    %q,\n"

  remapRule = `
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
%s
    "actual_binary": attr.label(cfg = _remap_transition),
    "_whitelist_function_transition": attr.label(
      default = "@bazel_tools//tools/whitelists/function_transition_whitelist",
    ),
  },
  # Making this executable means it works with "$ bazel run".
  executable = True,
)
`
  // A single line of the attrs block in remapRule.
  remapRuleAttr = "    %q: attr.label(),\n"

  nrfCCBinary = `
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
%s
  )
  cc_binary(
    name = cc_binary_name,
    **kwargs
  )
`
  // A single line of the _remap_rule block in nrfCCBinary.
  nrfCCBinaryRemapRule = "    %s = remap.get(%q, %q),\n"
)

// New creates a new remap from a list of header files from
// bazelifyrc.Configuration's remaps field.
// sdkFromWorkspace is the relative path from sdkDir to workspaceDir.
func New(headers []string, sdkFromWorkspace string) *Remaps {
  var libs []*buildfile.Library
  if len(headers) != 0 {
    libs = append(libs, &buildfile.Library{Name: emptyRemap})
  }
  var labelSettings []*buildfile.LabelSetting
  var remaps []*processed
  for _, header := range headers {
    shortName := strings.TrimSuffix(header, filepath.Ext(header))
    remapName := fmt.Sprintf("%s_remap", shortName)
    buildSettingDefault := fmt.Sprintf("//%s:%s", sdkFromWorkspace, emptyRemap)
    labelSettings = append(labelSettings, &buildfile.LabelSetting{
      Name: remapName,
      BuildSettingDefault: buildSettingDefault,
    })
    label := fmt.Sprintf("//%s", sdkFromWorkspace)
    if filepath.Base(sdkFromWorkspace) != remapName {
      label += fmt.Sprintf(":%s", remapName)
    }
    remaps = append(remaps, &processed{
      header: header,
      shortName: shortName,
      label: label,
      buildSettingDefault: buildSettingDefault,
    })
  }

  bzlContents := header
  bzlContents += generateRemapTransition(remaps)
  bzlContents += generateRemapRule(remaps)
  bzlContents += generateNrfCCBinary(remaps)

  return &Remaps{
    libs: libs,
    labelSettings: labelSettings,
    bzlContents: []byte(bzlContents),
  }
}

// GenerateLabel generates the label for remapping the header.
func GenerateLabel(header string, sdkDir string, workspaceDir string) (*bazel.Label, error) {
  shortName := strings.TrimSuffix(header, filepath.Ext(header))
  remapName := fmt.Sprintf("%s_remap", shortName)
	return bazel.NewLabel(sdkDir, remapName, workspaceDir)
}

type processed struct {
  // The original header name
  header string
  // The name of the file without the extension
  shortName string
  // The name of the remap label, which is //sdk_dir:short_name_remap
  label string
  // If no default is provided, the build setting default label to use.
  buildSettingDefault string
}

func generateRemapTransition(remaps []*processed) string {
  var returns string
  var outputs string
  for _, remap := range remaps {
    returns += fmt.Sprintf(remapTransitionReturn, remap.label, remap.shortName)
    outputs += fmt.Sprintf(remapTransitionOutput, remap.label)
  }
  return fmt.Sprintf(remapTransition, returns, outputs)
}

func generateRemapRule(remaps []*processed) string {
  var attrs string
  for _, remap := range remaps {
    attrs += fmt.Sprintf(remapRuleAttr, remap.shortName)
  }
  return fmt.Sprintf(remapRule, attrs)
}

func generateNrfCCBinary(remaps []*processed) string {
  var remapRules string
  for _, remap := range remaps {
    remapRules += fmt.Sprintf(nrfCCBinaryRemapRule, remap.shortName, remap.header, remap.buildSettingDefault)
  }
  return fmt.Sprintf(nrfCCBinary, remapRules)
}

// Remaps holds data for remapping header files dynamically.
type Remaps struct {
  libs []*buildfile.Library
  labelSettings []*buildfile.LabelSetting
  bzlContents []byte
}

// Libraries returns the libraries that need to be created.
func (r *Remaps) Libraries() []*buildfile.Library {
  return r.libs
}

// LabelSettings returns the label_attr rules that need to be created.
func (r *Remaps) LabelSettings() []*buildfile.LabelSetting {
  return r.labelSettings
}

// BzlContents returns the .bzl file's contents.
func (r *Remaps) BzlContents() []byte {
  return r.bzlContents
}
