load("@rules_cc//cc:defs.bzl", "cc_library")
package(default_visibility=["//visibility:public"])
cc_library(name="a", includes = ["."], deps = [":fbda828d-4e1c-4ed1-84c8-2d2a8b98eaea"])

cc_library(name="b", includes = ["."], deps = [":fbda828d-4e1c-4ed1-84c8-2d2a8b98eaea"])

cc_library(name="c", includes = ["."], deps = [":fbda828d-4e1c-4ed1-84c8-2d2a8b98eaea"])

cc_library(name="fbda828d-4e1c-4ed1-84c8-2d2a8b98eaea", hdrs = ["a.h", "b.h", "c.h"])

cc_library(name="nrfbazelify_empty_remap")

cc_library(name="sdk_config", hdrs = ["sdk_config.h"], includes = ["."])

label_setting(name="sdk_config_remap", build_setting_default="//example:nrfbazelify_empty_remap")
