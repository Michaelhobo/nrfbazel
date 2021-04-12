load("@rules_cc//cc:defs.bzl", "cc_library")
package(default_visibility=["//visibility:public"])
cc_library(name="79d8d6e3-ac40-4f03-9361-415ade6cb10b", hdrs = ["a.h", "b.h", "c.h"])

cc_library(name="a", includes = ["."], deps = [":79d8d6e3-ac40-4f03-9361-415ade6cb10b"])

cc_library(name="b", includes = ["."], deps = [":79d8d6e3-ac40-4f03-9361-415ade6cb10b"])

cc_library(name="c", includes = ["."], deps = [":79d8d6e3-ac40-4f03-9361-415ade6cb10b", ":b"])

cc_library(name="nrfbazelify_empty_remap")

cc_library(name="sdk_config", hdrs = ["sdk_config.h"], includes = ["."])

label_setting(name="sdk_config_remap", build_setting_default="//example:nrfbazelify_empty_remap")
