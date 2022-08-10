#!/usr/bin/env bash
# cleaning environment: prunning local networks, local orphan containers, ... etc
docker system prune

# stopping minikube cluster running locally
minikube stop 

# deleting minikube cluster
minikube delete