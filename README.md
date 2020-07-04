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
  named nrf_log_ctrl.c, and the header is named nrf_log.h, it won't match up the two.

### Setup

Add this to your WORKSPACE file:

```bzl
TODO
```

### Usage

```bash
bazel run //:nrfbazelify -- --workspace /your/repo/abs/path \
                            --sdk /your/repo/abs/path/nrf_sdk_dir
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