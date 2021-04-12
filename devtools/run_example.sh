#!/bin/bash
ROOT="$(realpath $(dirname $0)/..)"
SDK="${ROOT}/example"
DOT_GRAPH_PATH="${SDK}/depgraph.dot"
DOT_GRAPH_PROGRESSION_DIR="${SDK}/depgraph_progression_dot"
DOT_GRAPH_PROGRESSION_PNG_DIR="${SDK}/depgraph_progression"
rm -rf "${DOT_GRAPH_PROGRESSION_DIR}"
mkdir -p "${DOT_GRAPH_PROGRESSION_DIR}"
rm -rf "${DOT_GRAPH_PROGRESSION_PNG_DIR}"
mkdir -p "${DOT_GRAPH_PROGRESSION_PNG_DIR}"
bazel run //cmd/nrfbazelify -- --workspace "${ROOT}" --sdk "${SDK}" --dot_graph_path "${DOT_GRAPH_PATH}" --dot_graph_progression_dir "${DOT_GRAPH_PROGRESSION_DIR}"
dot -Tpng "${DOT_GRAPH_PATH}" > "${SDK}/depgraph.png"
for f in ${DOT_GRAPH_PROGRESSION_DIR}/*
do
  NAME="$(basename ${f} .dot)"
  dot -Tpng "${f}" > "${DOT_GRAPH_PROGRESSION_PNG_DIR}/${NAME}.png"
done