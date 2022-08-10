#!/usr/bin/env bash

set -e
# set minikube memory config to 8GB
minikube config set memory 8000

# start minikube cluster locally
minikube start

# configure skaffold to be compliant for minikube runs
skaffold config set --global local-cluster true

# When you run Minikube it’s running in a virtual machine, and in effect
# you are now running two parallel docker environments, the one on your
# local machine and the one in the virtual machine. What the eval command
# does is set a selection of environment variables such that your current
# terminal session is pointing to the docker environment in the virtual machine.
# This means all the images you have locally won’t be available and vice versa
eval $(minikube docker-env)

# build docker container which will automatically be stored in minikube container registry
docker build -t feelguuds/service:latest .

# loading image into minikube
minikube image load feelguuds/service:latest
