#! /usr/bin/env sh

set -e

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd -P)

# run the build
$SCRIPT_DIR/build.sh

# create the kind cluster
kind create cluster || true

# load the docker image
kind load docker-image test/service:latest

# run the deploy
$SCRIPT_DIR/deploy.sh

# run the tests
$SCRIPT_DIR/test.sh
