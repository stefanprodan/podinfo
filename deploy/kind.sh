#! /usr/bin/env sh

# create the kind cluster
kind create cluster --config=kind.yaml

# add certificate manager
kubectl apply -f https://github.com/jetstack/cert-manager/releases/download/v1.0.4/cert-manager.yaml

# wait for cert manager
kubectl rollout status --namespace cert-manager deployment/cert-manager --timeout=2m
kubectl rollout status --namespace cert-manager deployment/cert-manager-webhook --timeout=2m
kubectl rollout status --namespace cert-manager deployment/cert-manager-cainjector --timeout=2m

# # apply the secure webapp
kubectl apply -f ./secure/common
kubectl apply -f ./secure/backend
kubectl apply -f ./secure/frontend

# # wait for the podinfo frontend to come up
kubectl rollout status --namespace secure deployment/frontend --timeout=1m

# curl the endpoints (responds with info due to header regexp on route handler)
echo
echo "http enpdoint:"
echo "curl http://localhost"
echo
curl http://localhost

echo
echo "https (secure) enpdoint:"
echo "curl --insecure https://localhost"
echo
curl --insecure https://localhost
