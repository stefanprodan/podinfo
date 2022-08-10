#!/usr/bin/env bash
set -e
export POD_NAME=$(kubectl get pods --namespace default -l "app.kubernetes.io/name=service,app.kubernetes.io/instance=service" -o jsonpath="{.items[0].metadata.name}")
export CONTAINER_PORT=$(kubectl get pod --namespace default $POD_NAME -o jsonpath="{.spec.containers[0].ports[0].containerPort}")
kubectl --namespace default port-forward $POD_NAME 8080:$CONTAINER_PORT
