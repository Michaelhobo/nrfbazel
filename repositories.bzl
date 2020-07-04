""" Defines all dependencies used by this repository.

If you use nrfbazel, you should call nrfbazel_dependencies()
"""

load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")
load("@bazel_gazelle//:deps.bzl", "go_repository")

def nrfbazel_dependencies():
    """ Contains all dependencies used by the nrfbazel repository."""
    _maybe(
        http_archive,
        name = "io_bazel_rules_go",
        sha256 = "a8d6b1b354d371a646d2f7927319974e0f9e52f73a2452d2b3877118169eb6bb",
        urls = [
            "https://mirror.bazel.build/github.com/bazelbuild/rules_go/releases/download/v0.23.3/rules_go-v0.23.3.tar.gz",
            "https://github.com/bazelbuild/rules_go/releases/download/v0.23.3/rules_go-v0.23.3.tar.gz",
        ],
    )
    _maybe(
        http_archive,
        name = "com_google_protobuf",
        sha256 = "ffd7e4f2eb7864347bc165d2874766eefc6b2da89d29551a05fc88338cbed9fe",
        strip_prefix = "protobuf-master",
        urls = ["https://github.com/protocolbuffers/protobuf/archive/master.zip"],
    )

    go_repositories()

def go_repositories():
    """ Contains all go repositories used by this module."""
    go_repository(
        name = "com_github_golang_protobuf",
        importpath = "github.com/golang/protobuf",
        sum = "h1:+Z5KGCizgyZCbGh1KZqA0fcLLkwbsjIzS4aV2v7wJX0=",
        version = "v1.4.2",
    )

    go_repository(
        name = "com_github_google_go_cmp",
        importpath = "github.com/google/go-cmp",
        sum = "h1:xsAVV57WRhGj6kEIi8ReJzQlHHqcBYCElAvkovg3B/4=",
        version = "v0.4.0",
    )

    go_repository(
        name = "org_golang_google_protobuf",
        importpath = "google.golang.org/protobuf",
        sum = "h1:4MY060fB1DLGMB/7MBTLnwQUY6+F09GEiz6SsrNqyzM=",
        version = "v1.23.0",
    )

    go_repository(
        name = "org_golang_x_xerrors",
        importpath = "golang.org/x/xerrors",
        sum = "h1:E7g+9GITq07hpfrRu66IVDexMakfv52eLZ2CXBWiKr4=",
        version = "v0.0.0-20191204190536-9bdfabe68543",
    )

def _maybe(rule, name, **kwargs):
    if name not in native.existing_rules():
        rule(name = name, **kwargs)
