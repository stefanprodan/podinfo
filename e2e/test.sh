#!/usr/bin/env bash

set -o errexit

function finish {
  echo '>>> Test logs'
  kubectl logs -l app=podinfo
}
trap finish EXIT

echo '>>> Start integration tests'
helm test podinfo
