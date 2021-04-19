load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = [
        "config.go",
        "graph.go",
        "groups.go",
        "hint.go",
        "nodes.go",
        "nrfbazelify.go",
        "output.go",
        "walk.go",
    ],
    importpath = "github.com/Michaelhobo/nrfbazel/nrfbazelify",
    visibility = ["//visibility:public"],
    deps = [
        "//internal/bazel:go_default_library",
        "//internal/buildfile:go_default_library",
        "//internal/remap:go_default_library",
        "//proto/bazelifyrc:bazelifyrc_go_proto",
        "@com_github_google_uuid//:go_default_library",
        "@org_golang_google_protobuf//encoding/prototext:go_default_library",
        "@org_golang_google_protobuf//proto:go_default_library",
        "@org_gonum_v1_gonum//graph:go_default_library",
        "@org_gonum_v1_gonum//graph/encoding/dot:go_default_library",
        "@org_gonum_v1_gonum//graph/simple:go_default_library",
    ],
)

go_test(
    name = "go_default_test",
    srcs = ["nrfbazelify_test.go"],
    args = ["-test.v"],
    data = glob(["testdata/**"]),
    embed = [":go_default_library"],
    deps = [
        "//internal/buildfile:go_default_library",
        "//proto/bazelifyrc:bazelifyrc_go_proto",
        "@com_github_google_go_cmp//cmp:go_default_library",
        "@org_golang_google_protobuf//encoding/prototext:go_default_library",
        "@org_golang_google_protobuf//testing/protocmp:go_default_library",
    ],
)
