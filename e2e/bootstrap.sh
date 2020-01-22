#!/usr/bin/env bash

set -o errexit

REPO_ROOT=$(git rev-parse --show-toplevel)
KIND_VERSION=v0.7.0

if [[ "$1" ]]; then
  KIND_VERSION=$1
fi

echo ">>> Installing kubectl"
curl -sLO https://storage.googleapis.com/kubernetes-release/release/$(curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt)/bin/linux/amd64/kubectl && \
chmod +x kubectl && \
sudo mv kubectl /usr/local/bin/

echo ">>> Installing kind"
curl -sSLo kind "https://github.com/kubernetes-sigs/kind/releases/download/$KIND_VERSION/kind-linux-amd64"
chmod +x kind
sudo mv kind /usr/local/bin/kind

echo ">>> Creating kind cluster"
kind create cluster --wait 5m

echo ">>> Installing Helm v3"
curl https://raw.githubusercontent.com/helm/helm/master/scripts/get-helm-3 | bash
