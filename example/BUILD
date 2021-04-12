load("@rules_cc//cc:defs.bzl", "cc_library")
package(default_visibility=["//visibility:public"])
cc_library(name="05d56680-7342-4ae1-b8f2-253e740576d4", hdrs = ["a.h", "b.h", "c.h"])

cc_library(name="a", includes = ["."], deps = [":05d56680-7342-4ae1-b8f2-253e740576d4"])

cc_library(name="b", includes = ["."], deps = [":05d56680-7342-4ae1-b8f2-253e740576d4"])

cc_library(name="c", includes = ["."], deps = [":05d56680-7342-4ae1-b8f2-253e740576d4"])

cc_library(name="nrfbazelify_empty_remap")

cc_library(name="sdk_config", hdrs = ["sdk_config.h"], includes = ["."])

label_setting(name="sdk_config_remap", build_setting_default="//example:nrfbazelify_empty_remap")
