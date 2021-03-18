#!/bin/bash -x
bazel run //:gazelle -- update-repos --from_file go.mod
bazel run //:gazelle
