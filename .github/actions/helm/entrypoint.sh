#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

HELM_VERSION=3.2.1
BIN_DIR=/home/runner/bin

main() {
  mkdir -p ${BIN_DIR}
  tmpDir=$(mktemp -d)

  pushd $tmpDir >& /dev/null

  curl -sSL https://get.helm.sh/helm-v${HELM_VERSION}-linux-amd64.tar.gz | tar xz
  cp linux-amd64/helm ${BIN_DIR}/helm

  popd >& /dev/null
  rm -rf $tmpDir
}

main