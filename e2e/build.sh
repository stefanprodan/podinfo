#!/usr/bin/env bash

set -o errexit

GIT_COMMIT=$(git rev-list -1 HEAD)

docker build -t test/podinfo:latest --build-arg "REVISION=${GIT_COMMIT}" .

