workspace(name = "nrfbazel")

load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

http_archive(
    name = "io_bazel_rules_go",
    sha256 = "69de5c704a05ff37862f7e0f5534d4f479418afc21806c887db544a316f3cb6b",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/rules_go/releases/download/v0.27.0/rules_go-v0.27.0.tar.gz",
        "https://github.com/bazelbuild/rules_go/releases/download/v0.27.0/rules_go-v0.27.0.tar.gz",
    ],
)

load("@io_bazel_rules_go//go:deps.bzl", "go_register_toolchains", "go_rules_dependencies")

go_rules_dependencies()

go_register_toolchains(version = "1.16")

http_archive(
    name = "bazel_gazelle",
    sha256 = "62ca106be173579c0a167deb23358fdfe71ffa1e4cfdddf5582af26520f1c66f",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/bazel-gazelle/releases/download/v0.23.0/bazel-gazelle-v0.23.0.tar.gz",
        "https://github.com/bazelbuild/bazel-gazelle/releases/download/v0.23.0/bazel-gazelle-v0.23.0.tar.gz",
    ],
)

load("@bazel_gazelle//:deps.bzl", "gazelle_dependencies", "go_repository")

gazelle_dependencies()

http_archive(
    name = "golink",
    urls = ["https://github.com/nikunjy/golink/archive/v1.0.0.tar.gz"],
    sha256 = "ea728cfc9cb6e2ae024e1d5fbff185224592bbd4dad6516f3cc96d5155b69f0d",
    strip_prefix = "golink-1.0.0",
)

http_archive(
    name = "com_google_protobuf",
    sha256 = "ffd7e4f2eb7864347bc165d2874766eefc6b2da89d29551a05fc88338cbed9fe",
    strip_prefix = "protobuf-master",
    urls = ["https://github.com/protocolbuffers/protobuf/archive/master.zip"],
)

load("@com_google_protobuf//:protobuf_deps.bzl", "protobuf_deps")

protobuf_deps()

load("//:repositories.bzl", "nrfbazel_dependencies")

nrfbazel_dependencies()

go_repository(
    name = "com_github_golang_protobuf",
    importpath = "github.com/golang/protobuf",
    sum = "h1:jAbXjIeW2ZSW2AwFxlGTDoc2CjI2XujLkV3ArsZFCvc=",
    version = "v1.5.1",
)

go_repository(
    name = "com_github_google_go_cmp",
    importpath = "github.com/google/go-cmp",
    sum = "h1:Khx7svrCpmxxtHBq5j2mp/xVjsi8hQMfNLvJFAlrGgU=",
    version = "v0.5.5",
)

go_repository(
    name = "com_github_yuin_goldmark",
    importpath = "github.com/yuin/goldmark",
    sum = "h1:nqDD4MMMQA0lmWq03Z2/myGPYLQoXtmi0rGVs95ntbo=",
    version = "v1.1.27",
)

go_repository(
    name = "org_golang_google_protobuf",
    importpath = "google.golang.org/protobuf",
    sum = "h1:bxAC2xTBsZGibn2RTntX0oH50xLsqy1OxA9tTL3p/lk=",
    version = "v1.26.0",
)

go_repository(
    name = "org_golang_x_crypto",
    importpath = "golang.org/x/crypto",
    sum = "h1:ObdrDkeb4kJdCP557AjRjq69pTHfNouLtWZG7j9rPN8=",
    version = "v0.0.0-20191011191535-87dc89f01550",
)

go_repository(
    name = "org_golang_x_mod",
    importpath = "golang.org/x/mod",
    sum = "h1:KU7oHjnv3XNWfa5COkzUifxZmxp1TyI7ImMXqFxLwvQ=",
    version = "v0.2.0",
)

go_repository(
    name = "org_golang_x_net",
    importpath = "golang.org/x/net",
    sum = "h1:0mm1VjtFUOIlE1SbDlwjYaDxZVDP2S5ou6y0gSgXHu8=",
    version = "v0.0.0-20200226121028-0de0cce0169b",
)

go_repository(
    name = "org_golang_x_sync",
    importpath = "golang.org/x/sync",
    sum = "h1:vcxGaoTs7kV8m5Np9uUNQin4BrLOthgV7252N8V+FwY=",
    version = "v0.0.0-20190911185100-cd5d95a43a6e",
)

go_repository(
    name = "org_golang_x_sys",
    importpath = "golang.org/x/sys",
    sum = "h1:+R4KGOnez64A81RvjARKc4UT5/tI9ujCIVX+P5KiHuI=",
    version = "v0.0.0-20190412213103-97732733099d",
)

go_repository(
    name = "org_golang_x_text",
    importpath = "golang.org/x/text",
    sum = "h1:g61tztE5qeGQ89tm6NTjjM9VPIm088od1l6aSorWRWg=",
    version = "v0.3.0",
)

go_repository(
    name = "org_golang_x_tools",
    importpath = "golang.org/x/tools",
    sum = "h1:/e4fNMHdLn7SQSxTrRZTma2xjQW6ELdxcnpqMhpo9X4=",
    version = "v0.0.0-20200702044944-0cc1aa72b347",
)

go_repository(
    name = "org_golang_x_xerrors",
    importpath = "golang.org/x/xerrors",
    sum = "h1:E7g+9GITq07hpfrRu66IVDexMakfv52eLZ2CXBWiKr4=",
    version = "v0.0.0-20191204190536-9bdfabe68543",
)
