#!/usr/bin/env bash

set -o errexit

REPO_ROOT=$(git rev-parse --show-toplevel)
export KUBECONFIG="$(kind get kubeconfig-path --name="kind")"

function finish {
  echo '>>> Test logs'
  kubectl logs -l app=podinfo
}
trap finish EXIT

echo '>>> Testing'
helm test podinfo

