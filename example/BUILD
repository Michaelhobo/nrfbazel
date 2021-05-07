load("@bazel_skylib//rules:common_settings.bzl", "string_list_setting")
load("@rules_cc//cc:defs.bzl", "cc_library")

package(default_visibility = ["//visibility:public"])

cc_library(
    name = "a",
    defines = ":nrf_defines",
    deps = [":maingroup"],
)

cc_library(
    name = "b",
    defines = ":nrf_defines",
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
    copts = [
        "-Iexample",
        "-Iexample/dir",
        "-Iexample/dir2",
    ],
    defines = ":nrf_defines",
    deps = ["//example/dir2:used_by_cyclic"],
)

cc_library(
    name = "nrfbazelify_empty_remap",
    defines = ":nrf_defines",
)

cc_library(
    name = "sdk_config",
    hdrs = ["sdk_config.h"],
    defines = ":nrf_defines",
)

label_setting(
    name = "sdk_config_remap",
    build_setting_default = "//example:nrfbazelify_empty_remap",
)

string_list_setting(
    name = "nrf_defines",
    build_setting_default = [],
)
