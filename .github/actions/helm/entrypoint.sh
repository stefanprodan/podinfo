#!/usr/bin/env bash

set -o errexit
set -o pipefail

HELM_VERSION=$1
BIN_DIR="$GITHUB_WORKSPACE/bin"

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
echo "$GITHUB_WORKSPACE/bin" >> $GITHUB_PATH
echo "$RUNNER_WORKSPACE/$(basename $GITHUB_REPOSITORY)/bin" >> $GITHUB_PATH
