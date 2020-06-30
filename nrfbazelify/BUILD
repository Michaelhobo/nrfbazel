load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")
load("@io_bazel_rules_go//proto:def.bzl", "go_proto_library")
load("@rules_proto//proto:defs.bzl", "proto_library")

go_library(
    name = "go_default_library",
    srcs = ["nrfbazelify.go"],
    importpath = "github.com/Michaelhobo/nrfbazel/nrfbazelify",
    visibility = ["//visibility:public"],
    deps = [
        ":bazelifyrc_go_proto",
        "//buildfile",
        "@com_github_golang_protobuf//proto:go_default_library",
    ],
)

go_test(
    name = "go_default_test",
    srcs = ["nrfbazelify_test.go"],
    data = glob(["testdata/**"]),
    embed = [":go_default_library"],
    deps = ["//buildfile"],
)

proto_library(
    name = "bazelifyrc_proto",
    srcs = ["bazelifyrc.proto"],
    visibility = ["//visibility:public"],
)

go_proto_library(
    name = "bazelifyrc_go_proto",
    importpath = "github.com/Michaelhobo/nrfbazel/nrfbazelify/bazelifyrc",
    proto = ":bazelifyrc_proto",
    visibility = ["//visibility:public"],
)
