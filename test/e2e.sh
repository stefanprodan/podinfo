#! /usr/bin/env sh

set -e

CERT_MANAGER_CHART_VERSION="${CERT_MANAGER_CHART_VERSION:-v1.21.3}"
VALKEY_CHART_VERSION="${VALKEY_CHART_VERSION:-1.0.0}"

docker build --tag test/podinfo --build-arg "REVISION=0.0.0-$(git rev-list -1 HEAD)" .

kind load docker-image test/podinfo:latest

helm upgrade --install --wait cert-manager oci://quay.io/jetstack/charts/cert-manager \
    --version "${CERT_MANAGER_CHART_VERSION}" \
    --set crds.enabled=true \
    --namespace=cert-manager \
    --create-namespace

cat << 'EOF' | kubectl apply --server-side -f -
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: self-signed
spec:
  selfSigned: {}
EOF

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
    --set redis.tls.enabled=true \
    --namespace=default

helm test podinfo --logs

helm upgrade --install --wait valkey valkey \
    --repo https://valkey.io/valkey-helm/ \
    --version "${VALKEY_CHART_VERSION}" \
    --namespace=default

helm upgrade --install --wait podinfo-valkey ./charts/podinfo \
    --set image.repository=test/podinfo \
    --set image.tag=latest \
    --set cache=valkey://valkey:6379 \
    --namespace=default

helm test podinfo-valkey --logs
