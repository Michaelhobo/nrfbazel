load("@bazel_gazelle//:def.bzl", "DEFAULT_LANGUAGES", "gazelle", "gazelle_binary")

gazelle_binary(
    name = "gazelle_binary",
    languages = DEFAULT_LANGUAGES + ["@golink//gazelle/go_link:go_default_library"],
)

# gazelle:proto package
# gazelle:build_file_name BUILD
gazelle(
    name = "gazelle",
    command = "fix",
    prefix = "github.com/Michaelhobo/nrfbazel",
    gazelle = "//:gazelle_binary",
)
