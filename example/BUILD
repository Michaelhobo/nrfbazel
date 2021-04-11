load("@rules_cc//cc:defs.bzl", "cc_library")
package(default_visibility=["//visibility:public"])
cc_library(name="a", includes = ["."], deps = [":eb1e4a69-098a-4fb8-a6c6-31f723e3994d"])

cc_library(name="b", includes = ["."], deps = [":c", ":eb1e4a69-098a-4fb8-a6c6-31f723e3994d"])

cc_library(name="c", includes = ["."], deps = [":eb1e4a69-098a-4fb8-a6c6-31f723e3994d"])

cc_library(name="eb1e4a69-098a-4fb8-a6c6-31f723e3994d", hdrs = ["a.h", "b.h", "c.h"])

cc_library(name="nrfbazelify_empty_remap")

cc_library(name="sdk_config", hdrs = ["sdk_config.h"], includes = ["."])

label_setting(name="sdk_config_remap", build_setting_default="//example:nrfbazelify_empty_remap")
