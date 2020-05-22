#!/bin/bash
set -e

HELM_V3=3.2.1
BIN=/home/runner/bin
curl -sSL https://get.helm.sh/helm-v${HELM_V3}-linux-amd64.tar.gz | tar xz
mkdir -p ${BIN}
cp linux-amd64/helm ${BIN}/helm
rm -rf linux-amd64

