#!/bin/bash
ROOT="$(realpath $(dirname $0)/..)"
SDK="${ROOT}/example"
DOT_GRAPH_PATH="${SDK}/depgraph.dot"
bazel run //cmd/nrfbazelify -- --workspace ${ROOT} --sdk ${SDK} --dot_graph_path=${DOT_GRAPH_PATH}
dot -Tpng "${DOT_GRAPH_PATH}" > "${SDK}/depgraph.png"