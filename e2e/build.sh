#!/usr/bin/env bash

set -o errexit

docker build -t test/podinfo:latest .

