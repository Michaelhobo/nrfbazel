load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = ["label.go"],
    importpath = "github.com/Michaelhobo/nrfbazel/internal/bazel",
    visibility = ["//:__subpackages__"],
)

go_test(
    name = "go_default_test",
    srcs = ["label_test.go"],
    args = ["-test.v"],
    embed = [":go_default_library"],
)
