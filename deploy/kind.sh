#! /usr/bin/env sh

# create the kind cluster
kind create cluster --config=kind.yaml

# add certificate manager
kubectl apply -f https://github.com/jetstack/cert-manager/releases/download/v1.0.4/cert-manager.yaml

# wait for cert manager webhook
kubectl wait --namespace cert-manager \
  --for=condition=available deployment \
  --selector=app=webhook \
  --timeout=120s

# wait for the injector
kubectl wait --namespace cert-manager \
  --for=condition=available deployment \
  --selector=app=cainjector \
  --timeout=120s

# wait for the cert manager
kubectl wait --namespace cert-manager \
  --for=condition=available deployment \
  --selector=app=cert-manager \
  --timeout=120s

# apply the secure webapp
kubectl apply -f ./secure/common
kubectl apply -f ./secure/backend
kubectl apply -f ./secure/frontend

# wait for the podinfo frontend to come up
kubectl wait --namespace secure \
  --for=condition=ready pod \
  --selector=app=frontend \
  --timeout=120s

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
