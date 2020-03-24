#!/usr/bin/env bash

set -o errexit

function finish {
  echo '>>> Test logs'
  kubectl logs -l app=podinfo || true
}
trap "finish" EXIT SIGINT

echo '>>> Start integration tests'
helm test podinfo
