load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["main.go"],
    importpath = "github.com/Michaelhobo/nrfbazel/cmd/nrfbazelify",
    visibility = ["//visibility:private"],
    deps = ["//nrfbazelify:go_default_library"],
)

go_binary(
    name = "nrfbazelify",
    embed = [":go_default_library"],
    visibility = ["//visibility:public"],
)
