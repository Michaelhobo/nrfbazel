load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = ["nrfbazelify.go"],
    importpath = "github.com/Michaelhobo/nrfbazel/nrfbazelify",
    visibility = ["//visibility:public"],
    deps = [
        "//bazelifyrc:bazelifyrc_go_proto",
        "//internal/buildfile",
        "@com_github_golang_protobuf//proto:go_default_library",
    ],
)

go_test(
    name = "go_default_test",
    srcs = ["nrfbazelify_test.go"],
    data = glob(["testdata/**"]),
    embed = [":go_default_library"],
    deps = [
        "//bazelifyrc:bazelifyrc_go_proto",
        "//internal/buildfile",
        "@com_github_golang_protobuf//proto:go_default_library",
        "@com_github_google_go_cmp//cmp:go_default_library",
        "@org_golang_google_protobuf//testing/protocmp:go_default_library",
    ],
)
