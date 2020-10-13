# nrfbazel

This contains a collection of tools used for working with the nrf52 SDK with 
[Bazel](https://bazel.build).

## nrfbazelify

I created this tool to make converting the nrf52 SDK to Bazel easier. It is
*not* meant to flawlessly generate C/C++ BUILD files for every scenario. I 
take advantage of many aspects of the nrf52 SDK that make my life easier. 
I also took a few shortcuts that might result in incorrect but decently close
output, and the rest will be done by hand.

Some of these assumptions include:

* We *DELETE* all BUILD files in the SDK tree. This is meant to convert a 
  fresh SDK to Bazel, not meant for helping to maintain it.
* We expect a matching .c file for every .h file - e.g. if the src file is
  named nrf_log_ctrl.c, and the header is named nrf_log.h, it won't match up the
  two.

### Status & Planned Work

We have a few known limitations that make nrfbazelify difficult to use. I plan
on tackling each issue, when I have time. I plan to eventually put a permissive
license on this code, but I'd like to get it into a better state first.

#### Per-cc_binary includes

Some includes change depending on which cc_binary you're building from. For
example, if you have multiple cc_binary rules that use different softdevices,
we don't support that right now. This also makes the sdk_config.h difficult,
since developers often have a different sdk_config.h for each cc_binary target.

To solve this, I plan on using a Bazel
[transition](https://docs.bazel.build/versions/master/skylark/lib/transition.html)
rule, but I am thinking about ways to make this process simpler and more
generic.

#### Non-1:1 header and source file names

We only support 1:1 .h and .c file names. There are some cases where the .c
file is named differently, or there are multiple .c files for a single .h file.

For example, things like this break:

* `a.h` and `a_impl.c`
* `a.h` and `a.c` and `a_component.c`

I could potentially solve this by searching for .c files that include the .h in
the same directory, but that has many pitfalls. Or, I could search .c files for
implementations of the functions in the headers, but that is a much more
difficult problem. The simplest solution is to provide a different kind of
override in `.bazelifyrc`.

### Setup

Add this to your WORKSPACE file:

```bzl
http_archive(
    name = "io_bazel_rules_go",
    sha256 = "a8d6b1b354d371a646d2f7927319974e0f9e52f73a2452d2b3877118169eb6bb",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/rules_go/releases/download/v0.23.3/rules_go-v0.23.3.tar.gz",
        "https://github.com/bazelbuild/rules_go/releases/download/v0.23.3/rules_go-v0.23.3.tar.gz",
    ],
)

load("@io_bazel_rules_go//go:deps.bzl", "go_register_toolchains", "go_rules_dependencies")

go_rules_dependencies()

go_register_toolchains()

http_archive(
    name = "bazel_gazelle",
    sha256 = "cdb02a887a7187ea4d5a27452311a75ed8637379a1287d8eeb952138ea485f7d",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/bazel-gazelle/releases/download/v0.21.1/bazel-gazelle-v0.21.1.tar.gz",
        "https://github.com/bazelbuild/bazel-gazelle/releases/download/v0.21.1/bazel-gazelle-v0.21.1.tar.gz",
    ],
)

load("@bazel_gazelle//:deps.bzl", "gazelle_dependencies")

gazelle_dependencies()

http_archive(
    name = "com_google_protobuf",
    sha256 = "ffd7e4f2eb7864347bc165d2874766eefc6b2da89d29551a05fc88338cbed9fe",
    strip_prefix = "protobuf-master",
    urls = ["https://github.com/protocolbuffers/protobuf/archive/master.zip"],
)

load("@com_google_protobuf//:protobuf_deps.bzl", "protobuf_deps")
protobuf_deps()

http_archive(
  name = "nrfbazel",
  strip_prefix = "nrfbazel-master",
  urls = ["https://github.com/michaelhobo/nrfbazel/archive/master.zip"],
)

load("@nrfbazel//:repositories.bzl", "nrfbazel_dependencies")

nrfbazel_dependencies()
```

### Usage

```bash
bazel run @nrfbazel//cmd/nrfbazelify --
    --workspace $(realpath <workspace dir>) \
    --sdk $(realpath <sdk dir>)
```

**This will delete all BUILD files in /your/repo/abs/path/nrf_sdk_dir, and generate new ones.**

The tool does not do a very good job with formatting. Run buildifier after
nrfbazelify.

```bash
buildifier -r path/to/nrf_sdk_dir
```

### Handling Unresolved Dependencies

The nrf5 SDK includes all header files with a relative import (e.g. nrf_log.h),
and many imports can have multiple files that resolve to it. To solve this, we
scan all directories for header files that match, and nrfbazelify expects to
find exactly 1 matching header file in the SDK.

If nrfbazelify cannot satisfy all required dependencies or does not know which
dependency to use, it will look to the .bazelifyrc file in the root of the
SDK directory.

#### .bazelifyrc syntax

The .bazelifyrc file is a textproto representation of the
[Configuration](bazelifyrc/bazelifyrc.proto) message. 

There are two main ways to cut down on unresolved dependencies. First, you can
exclude any directories or files that you don't want. Just specify a set of
excludes in the file, like this:

```
excludes: "a/b/c"
excludes: "a/b/d/*"
```

Paths should be relative to the root of the SDK directory, and match syntax is
based on Go's [filepath.Match](https://golang.org/pkg/path/filepath/#Match).

**I recommend excluding the examples directory**

If you want to always resolve a header using a target that's outside the SDK,
you can manually override headers with target_overrides. You will probalby need
this feature to make the sdk_config.h work.

```
target_overrides <
  key: "c.h"
  value: "//path/to/target:c"
>
target_overrides <
  key: "d.h"
  value: "//path/to/d:d"
>
```

### Outstanding Issues

These are issues and problems which exist, but don't have planned solutions
yet. Most of these involve manual work to make the output usable.

#### Includes guarded by ifdefs, etc

Right now, we read all `#include` statements in source and header files.
However, some of them are guarded by #ifdefs or other guards. We don't solve
for this - we just ingest all #includes, and users will have to manually prune
the BUILD files.
