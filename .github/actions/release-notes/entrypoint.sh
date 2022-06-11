#!/usr/bin/env bash

set -o errexit
set -o pipefail

VERSION=0.2.0
BIN_DIR="$GITHUB_WORKSPACE/bin"

main() {
  mkdir -p ${BIN_DIR}
  tmpDir=$(mktemp -d)

  pushd $tmpDir >& /dev/null

  curl -sSL https://github.com/buchanae/github-release-notes/releases/download/${VERSION}/github-release-notes-linux-amd64-${VERSION}.tar.gz | tar xz
  cp github-release-notes ${BIN_DIR}/github-release-notes

  popd >& /dev/null
  rm -rf $tmpDir
}

main

echo "$BIN_DIR" >> $GITHUB_PATH
echo "$RUNNER_WORKSPACE/$(basename $GITHUB_REPOSITORY)/bin" >> $GITHUB_PATH
