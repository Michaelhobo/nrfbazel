load("@rules_proto//proto:defs.bzl", "proto_library")
load("@io_bazel_rules_go//proto:def.bzl", "go_proto_library")

proto_library(
    name = "bazelifyrc_proto",
    srcs = ["bazelifyrc.proto"],
    visibility = ["//visibility:public"],
)

go_proto_library(
    name = "bazelifyrc_go_proto",
    importpath = "github.com/Michaelhobo/nrfbazel/bazelifyrc",
    proto = ":bazelifyrc_proto",
    visibility = ["//visibility:public"],
)
