#!/usr/bin/env bash

set -o errexit

REPO_ROOT=$(git rev-parse --show-toplevel)

echo '>>> Loading image in Kind'
kind load docker-image test/podinfo:latest

echo '>>> Installing'
helm upgrade -i podinfo ${REPO_ROOT}/charts/podinfo --namespace=default
kubectl set image deployment/podinfo podinfo=test/podinfo:latest
kubectl rollout status deployment/podinfo
