#!/usr/bin/env bash

# watch the java files and continously deploy the service
make linux
skaffold run -p dev
reflex -r "\.go$" -- bash -c 'make linux && skaffold run -p dev'
