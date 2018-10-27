#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail
set -e

APP_DIR="/go/src/github.com/${GITHUB_REPOSITORY}/"

mkdir -p ${APP_DIR} && cp -r ./ ${APP_DIR} && cd ${APP_DIR}

if [[ "$1" == "fmt" ]]; then
    echo "Running go fmt"
    files=$(gofmt -l $(find . -type f -name '*.go' -not -path "./vendor/*") 2>&1)
    if [ "$files" ]; then
      echo "These files did not pass the gofmt check:"
      echo ${files}
      exit 1
    fi
fi

if [[ "$1" == "test" ]]; then
    echo "Running go test"
    go test $(go list ./... | grep -v /vendor/) -race -coverprofile=coverage.txt -covermode=atomic
    cat coverage.txt
fi
