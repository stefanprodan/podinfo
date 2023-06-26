#! /usr/bin/env sh

# install cert-manager
kubectl apply -f https://github.com/jetstack/cert-manager/releases/download/v1.5.3/cert-manager.yaml

# wait for cert manager
kubectl -n cert-manager rollout status deployment/cert-manager --timeout=2m
kubectl -n cert-manager rollout status deployment/cert-manager-webhook --timeout=2m
kubectl -n cert-manager rollout status deployment/cert-manager-cainjector --timeout=2m

# install self-signed certificate
cat << 'EOF' | kubectl apply -f -
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: self-signed
spec:
  selfSigned: {}
EOF

# install podinfo with tls enabled
helm upgrade --install podinfo ./charts/podinfo \
    --set image.repository=test/podinfo \
    --set image.tag=latest \
    --set tls.enabled=true \
    --set certificate.create=true \
    --set hpa.enabled=true \
    --set hpa.cpu=95 \
    --namespace=default
