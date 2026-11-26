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
helm test podinfo

# Install Valkey
cat << 'EOF' | kubectl apply --server-side -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: valkey
  namespace: default
spec:
  selector:
    matchLabels:
      app: valkey
  template:
    metadata:
      labels:
        app: valkey
    spec:
      containers:
        - name: valkey
          image: docker.io/valkey/valkey:9
          ports:
            - name: valkey
              containerPort: 6379
          readinessProbe:
            exec:
              command:
                - valkey-cli
                - ping
---
apiVersion: v1
kind: Service
metadata:
  name: valkey
  namespace: default
spec:
  selector:
    app: valkey
  ports:
    - name: valkey
      port: 6379
      targetPort: valkey
EOF
kubectl -n default rollout status deployment/valkey --timeout=2m

# Install podinfo with Valkey
helm upgrade --install --wait podinfo-valkey ./charts/podinfo \
    --set image.repository=test/podinfo \
    --set image.tag=latest \
    --set cache=valkey://valkey:6379 \
    --namespace=default

# Run tests
helm test podinfo-valkey
