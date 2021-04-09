load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["remap.go"],
    importpath = "github.com/Michaelhobo/nrfbazel/internal/remap",
    visibility = ["//nrfbazelify:__subpackages__"],
    deps = ["//internal/buildfile:go_default_library"],
)
