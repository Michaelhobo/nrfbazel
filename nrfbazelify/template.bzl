
""" This configures the nrf52 sdk_config.h as a transition.
"""
load("@rules_cc//cc:defs.bzl", "cc_binary")

def _remap_transition_impl(settings, attr):
    # settings provides read access to existing flags. But
    # this transition doesn't need to read any flags.
    return {
      "//:sdk_config": attr.sdk_config,
      "//:nrfx_config": attr.nrfx_config,
    }

_remap_transition = transition(
    implementation = _remap_transition_impl,
    inputs = [],
    outputs = ["//:sdk_config", "//:nrfx_config"],
)

# The implementation of transition_rule: all this does is copy the
# cc_binary's output to its own output and propagate its runfiles
# and executable to use for "$ bazel run".
#
# This makes transition_rule as close to a pure wrapper of cc_binary
# as possible.
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

# Enable us to configure an sdk_config attribute.
_remap_rule = rule(
    implementation = _remap_rule_impl,
    attrs = {
        "sdk_config": attr.label(),
        "nrfx_config": attr.label(),
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
        sdk_config = remap["sdk_config.h"],
        nrfx_config = remap["nrfx_config.h"],
    )
    cc_binary(
        name = cc_binary_name,
        **kwargs
    )