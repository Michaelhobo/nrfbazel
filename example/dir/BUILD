load("@rules_cc//cc:defs.bzl", "cc_library")

package(default_visibility = ["//visibility:public"])

exports_files(["c.h"])

cc_library(
    name = "c",
    deps = ["//example:maingroup"],
)

cc_library(
    name = "uses_cyclic",
    hdrs = ["uses_cyclic.h"],
    copts = ["-Iexample/dir"],
    deps = [":c"],
)
