load("@rules_cc//cc:defs.bzl", "cc_library")

package(default_visibility = ["//visibility:public"])

cc_library(
    name = "a",
    deps = [":maingroup"],
)

cc_library(
    name = "b",
    deps = [":maingroup"],
)

cc_library(
    name = "maingroup",
    hdrs = [
        "a.h",
        "b.h",
        "//example/dir:c.h",
        "//example/dir2:d.h",
    ],
    includes = [
        "example",
        "example/dir",
        "example/dir2",
    ],
    deps = ["//example/dir2:used_by_cyclic"],
)

cc_library(name = "nrfbazelify_empty_remap")

cc_library(
    name = "sdk_config",
    hdrs = ["sdk_config.h"],
    includes = ["example"],
)

label_setting(
    name = "sdk_config_remap",
    build_setting_default = "//example:nrfbazelify_empty_remap",
)
