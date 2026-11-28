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

kind create cluster --config=./bin/kind.yaml

kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.21.3/cert-manager.yaml

kubectl rollout status --namespace cert-manager deployment/cert-manager --timeout=2m
kubectl rollout status --namespace cert-manager deployment/cert-manager-webhook --timeout=2m
kubectl rollout status --namespace cert-manager deployment/cert-manager-cainjector --timeout=2m

# wait for the webhook CA to be injected
until kubectl apply --dry-run=server -f ./secure/common/cluster-issuer.yaml >/dev/null 2>&1; do
  echo "waiting for cert-manager webhook..."
  sleep 5
done

kubectl apply -f ./secure/common
kubectl apply -f ./secure/backend
kubectl apply -f ./secure/frontend

kubectl rollout status --namespace secure deployment/frontend --timeout=3m

# curl gets the JSON info: the UI is served only to a browser User-Agent
echo
echo "http endpoint:"
echo "curl http://localhost"
echo
curl http://localhost

echo
echo "https (secure) endpoint:"
echo "curl --insecure https://localhost"
echo
curl --insecure https://localhost
