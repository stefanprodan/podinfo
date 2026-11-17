#! /usr/bin/env sh
set -e

# run relative to this script
cd "$(dirname "$0")"

mkdir -p bin
cat > ./bin/kind.yaml <<EOF
apiVersion: kind.x-k8s.io/v1alpha4
kind: Cluster
nodes:
- role: control-plane
  extraPortMappings:
  - containerPort: 80
    hostPort: 80
    protocol: TCP
  - containerPort: 443
    hostPort: 443
    protocol: TCP
EOF

# create the kind cluster
kind create cluster --config=./bin/kind.yaml

# add certificate manager
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.21.3/cert-manager.yaml

# wait for cert manager
kubectl rollout status --namespace cert-manager deployment/cert-manager --timeout=2m
kubectl rollout status --namespace cert-manager deployment/cert-manager-webhook --timeout=2m
kubectl rollout status --namespace cert-manager deployment/cert-manager-cainjector --timeout=2m

# wait for the webhook CA to be injected
until kubectl apply --dry-run=server -f ./secure/common/cluster-issuer.yaml >/dev/null 2>&1; do
  echo "waiting for cert-manager webhook..."
  sleep 5
done

# # apply the secure webapp
kubectl apply -f ./secure/common
kubectl apply -f ./secure/backend
kubectl apply -f ./secure/frontend

# # wait for the podinfo frontend to come up
kubectl rollout status --namespace secure deployment/frontend --timeout=3m

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
