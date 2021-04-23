#!/bin/bash -e
ROOT="$(realpath $(dirname $0)/..)"
SDK="${ROOT}/example"
bazel run //cmd/nrfbazelify -- \
  --workspace "${ROOT}" --sdk "${SDK}" \
  --full_graph --progression_graphs --named_group_graphs

buildifier -r "${SDK}"

DOT_OUT=${SDK}/.bazelify-out/dot
PNG_OUT=${SDK}/.bazelify-out/png
rm -rf "${PNG_OUT}"
mkdir -p "${PNG_OUT}"
for f in ${DOT_OUT}/*/*.dot
do
  OFFSET=${f#$DOT_OUT}
  OFFSET=${OFFSET%dot}
  DIR=$(dirname ${PNG_OUT}${OFFSET})
  mkdir -p ${DIR}
  dot -Tpng "${f}" > "${PNG_OUT}${OFFSET}png"
done
