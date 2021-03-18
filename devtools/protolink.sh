#!/bin/bash
# This generates the proto symlinks to allow Intellisense to find the proto files.
for t in $(bazel query 'kind("proto_link", //...)'); do
  bazel run $t
done
