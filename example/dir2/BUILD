load("@rules_cc//cc:defs.bzl", "cc_library")

package(default_visibility = ["//visibility:public"])

exports_files(["d.h"])

cc_library(
    name = "d",
    deps = ["//example:maingroup"],
)

cc_library(
    name = "used_by_cyclic",
    hdrs = ["used_by_cyclic.h"],
)
