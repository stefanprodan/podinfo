#! /usr/bin/env sh

set -e

# Build container image
docker build --tag test/podinfo --build-arg "REVISION=0.0.0-$(git rev-list -1 HEAD)" .

# Load image in cluster
kind load docker-image test/podinfo:latest

# Install cert-manager
kubectl apply --server-side -f https://github.com/cert-manager/cert-manager/releases/download/v1.20.2/cert-manager.yaml
kubectl -n cert-manager rollout status deployment/cert-manager --timeout=2m
kubectl -n cert-manager rollout status deployment/cert-manager-webhook --timeout=2m
kubectl -n cert-manager rollout status deployment/cert-manager-cainjector --timeout=2m

# Configure self-signed certificate
cat << 'EOF' | kubectl apply --server-side -f -
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: self-signed
spec:
  selfSigned: {}
EOF

# Install podinfo with TLS and Redis enabled
helm upgrade --install --wait podinfo ./charts/podinfo \
    --set image.repository=test/podinfo \
    --set image.tag=latest \
    --set tls.enabled=true \
    --set certificate.create=true \
    --set hpa.enabled=true \
    --set hpa.cpu=95 \
    --set replicaCount=2 \
    --set hooks.postInstall.job.enabled=true \
    --set redis.enabled=true \
    --namespace=default

# Run tests
helm test podinfo --logs

# Install Valkey
helm upgrade --install --wait valkey valkey \
    --repo https://valkey.io/valkey-helm/ \
    --version 1.0.0 \
    --namespace=default

# Install podinfo with Valkey
helm upgrade --install --wait podinfo-valkey ./charts/podinfo \
    --set image.repository=test/podinfo \
    --set image.tag=latest \
    --set cache=valkey://valkey:6379 \
    --namespace=default

# Run tests
helm test podinfo-valkey --logs
