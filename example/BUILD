load("@rules_cc//cc:defs.bzl", "cc_library")
package(default_visibility=["//visibility:public"])
cc_library(name="a", includes = ["."], deps = [":a1f1b8c8-4d62-4ee7-9ad0-c3df1651eb53"])

cc_library(name="a1f1b8c8-4d62-4ee7-9ad0-c3df1651eb53", hdrs = ["a.h", "b.h", "c.h"])

cc_library(name="b", includes = ["."], deps = [":a1f1b8c8-4d62-4ee7-9ad0-c3df1651eb53", ":c"])

cc_library(name="c", includes = ["."], deps = [":a1f1b8c8-4d62-4ee7-9ad0-c3df1651eb53"])

cc_library(name="nrfbazelify_empty_remap")

cc_library(name="sdk_config", hdrs = ["sdk_config.h"], includes = ["."])

label_setting(name="sdk_config_remap", build_setting_default="//example:nrfbazelify_empty_remap")
