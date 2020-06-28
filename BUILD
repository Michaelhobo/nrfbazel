load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library")

go_library(
    name = "nrfbazelify_lib",
    srcs = ["nrfbazelify.go"],
    importpath = "github.com/Michaelhobo/nrfbazel/nrfbazelify",
    deps = [":buildfile"],
)

go_library(
    name = "buildfile",
    srcs = ["buildfile.go"],
    importpath = "github.com/Michaelhobo/nrfbazel/buildfile",
)

go_binary(
    name = "nrfbazelify",
    srcs = ["nrfbazelify_main.go"],
    visibility = ["//visibility:public"],
    deps = [":nrfbazelify_lib"],
)
