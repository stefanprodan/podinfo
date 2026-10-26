#!/usr/bin/env bash
# End-to-end test for podinfo SPIFFE mTLS with SPIRE on a kind cluster.
set -euo pipefail

NAMESPACE="podinfo"
SPIRE_CRDS_CHART_VERSION="${SPIRE_CRDS_CHART_VERSION:-0.6.1}"
SPIRE_CHART_VERSION="${SPIRE_CHART_VERSION:-0.30.3}"
SPIFFE_DOMAIN="example.org"
PODINFO_ID="spiffe://${SPIFFE_DOMAIN}/ns/${NAMESPACE}/sa/podinfo"
OTHER_ID="spiffe://${SPIFFE_DOMAIN}/ns/${NAMESPACE}/sa/other"

log()  { printf '\n\033[1;34m==> %s\033[0m\n' "$*"; }
info() { printf '    %s\n' "$*"; }
fail() { printf '\033[1;31mFAIL: %s\033[0m\n' "$*" >&2; exit 1; }

cleanup() {
    local code=$?
    if [[ $code -ne 0 ]]; then
        log "FAILURE (exit ${code}) - dumping diagnostics"
        kubectl -n "${NAMESPACE}" get pods -o wide || true
        kubectl -n "${NAMESPACE}" describe pods || true
        kubectl -n "${NAMESPACE}" logs deploy/podinfo --tail=100 || true
        kubectl -n "${NAMESPACE}" logs pod/other-client --all-containers || true
        kubectl get clusterspiffeids -o yaml || true
        kubectl -n spire-system get pods -o wide || true
        kubectl -n spire-server logs -l app.kubernetes.io/name=server --all-containers --tail=50 || true
    fi
    exit "$code"
}
trap cleanup EXIT

for bin in kubectl helm kind docker jq; do
    command -v "${bin}" >/dev/null 2>&1 || fail "required tool '${bin}' not found"
done

log "Building podinfo"
docker build --tag test/podinfo --build-arg "REVISION=0.0.0-$(git rev-list -1 HEAD)" .
kind load docker-image test/podinfo:latest

log "Installing SPIRE"
helm repo add spiffe https://spiffe.github.io/helm-charts-hardened >/dev/null 2>&1 || true
helm repo update spiffe >/dev/null
helm upgrade --install spire-crds spiffe/spire-crds \
    --version "${SPIRE_CRDS_CHART_VERSION}" \
    --namespace spire-crds \
    --create-namespace \
    --wait --timeout 5m
# The SPIRE chart creates its own namespaces. Pre-create them and keep the
# chart's namespace creation disabled to avoid a conflict with Helm 4's
# --create-namespace behavior.
kubectl create namespace spire-system --dry-run=client -o yaml | kubectl apply -f - >/dev/null
kubectl create namespace spire-server --dry-run=client -o yaml | kubectl apply -f - >/dev/null
# The default ClusterSPIFFEID issues an identity to every pod. Disable it
# so only the ClusterSPIFFEID created below issues identities, which lets
# the test check podinfo before and after it gets an SVID.
helm upgrade --install spire spiffe/spire \
    --version "${SPIRE_CHART_VERSION}" \
    --namespace spire-system \
    --set global.spire.trustDomain="${SPIFFE_DOMAIN}" \
    --set global.spire.namespaces.create=false \
    --set spire-server.controllerManager.enabled=true \
    --set spire-server.controllerManager.identities.clusterSPIFFEIDs.default.enabled=false \
    --set spiffe-csi-driver.enabled=true \
    --set spiffe-oidc-discovery-provider.enabled=false \
    --wait --timeout 10m
kubectl -n spire-system wait --for=condition=Ready pods --all --timeout=3m

log "Installing podinfo before its ClusterSPIFFEID exists"
kubectl create namespace "${NAMESPACE}" --dry-run=client -o yaml | kubectl apply -f - >/dev/null
helm upgrade --install podinfo ./charts/podinfo \
    --namespace "${NAMESPACE}" \
    --set fullnameOverride=podinfo \
    --set image.repository=test/podinfo \
    --set image.tag=latest \
    --set logLevel=debug \
    --set serviceAccount.enabled=true \
    --set tls.enabled=true \
    --set spiffe.enabled=true \
    --set "spiffe.authorizedIDs={${PODINFO_ID}}"

kubectl -n "${NAMESPACE}" wait --for=jsonpath='{.status.phase}'=Running pod -l app.kubernetes.io/name=podinfo --timeout=3m
# give the probes time to run: liveness must pass, readiness must not
sleep 30
POD=$(kubectl -n "${NAMESPACE}" get pod -l app.kubernetes.io/name=podinfo -o jsonpath='{.items[0].metadata.name}')
[[ "$(kubectl -n "${NAMESPACE}" get pod "${POD}" -o jsonpath='{.status.containerStatuses[0].ready}')" == "false" ]] \
    || fail "podinfo is ready without an SVID"
[[ "$(kubectl -n "${NAMESPACE}" get pod "${POD}" -o jsonpath='{.status.containerStatuses[0].restartCount}')" == "0" ]] \
    || fail "podinfo restarted while waiting for an SVID"
kubectl -n "${NAMESPACE}" logs "${POD}" | jq -e 'select(.msg | test("Workload API"))' >/dev/null \
    || fail "podinfo did not log the Workload API wait"
info "podinfo is live but not ready without an SVID"

log "Registering the podinfo workloads"
# The chart's controller manager only processes ClusterSPIFFEIDs with its
# class name (<namespace>-<release>), classless ones are ignored.
kubectl apply -f - <<EOF
apiVersion: spire.spiffe.io/v1alpha1
kind: ClusterSPIFFEID
metadata:
  name: podinfo
spec:
  className: spire-system-spire
  spiffeIDTemplate: "spiffe://{{ .TrustDomain }}/ns/{{ .PodMeta.Namespace }}/sa/{{ .PodSpec.ServiceAccountName }}"
  podSelector: {}
  namespaceSelector:
    matchLabels:
      kubernetes.io/metadata.name: ${NAMESPACE}
EOF

kubectl -n "${NAMESPACE}" wait --for=condition=Ready pod "${POD}" --timeout=3m
[[ "$(kubectl -n "${NAMESPACE}" get pod "${POD}" -o jsonpath='{.status.containerStatuses[0].restartCount}')" == "0" ]] \
    || fail "podinfo restarted before getting an SVID"
kubectl -n "${NAMESPACE}" logs "${POD}" | jq -e --arg id "${PODINFO_ID}" \
    'select(.msg == "Starting HTTPS Server with SPIFFE mTLS" and .spiffe_id == $id)' >/dev/null \
    || fail "podinfo did not start the HTTPS server with its SVID"
info "podinfo became ready with ${PODINFO_ID} without a restart"

log "Checking the artifact served from the image"
DIGEST=$(kubectl -n "${NAMESPACE}" exec "${POD}" -- curl -sfI http://localhost:9898/artifact \
    | tr -d '\r' | awk -F': ' 'tolower($1)=="x-artifact-digest" {print $2}')
[[ "${DIGEST}" == sha256:* ]] || fail "missing X-Artifact-Digest, got '${DIGEST}'"
BODY_DIGEST="sha256:$(kubectl -n "${NAMESPACE}" exec "${POD}" -- sh -c 'curl -sf http://localhost:9898/artifact | sha256sum' | awk '{print $1}')"
[[ "${DIGEST}" == "${BODY_DIGEST}" ]] || fail "digest header ${DIGEST} != body digest ${BODY_DIGEST}"
info "artifact digest ${DIGEST}"

log "Running the Helm tests, including the SPIFFE client checks"
helm test podinfo --namespace "${NAMESPACE}" --logs
kubectl -n "${NAMESPACE}" logs "${POD}" | jq -e --arg id "${PODINFO_ID}" \
    'select(.msg == "request started" and .spiffe_id == $id)' >/dev/null \
    || fail "podinfo did not log the client SPIFFE ID"
info "podinfo logged the client SPIFFE ID"

log "Checking that an SVID not in authorizedIDs is rejected"
kubectl -n "${NAMESPACE}" create serviceaccount other --dry-run=client -o yaml | kubectl apply -f - >/dev/null
kubectl -n "${NAMESPACE}" delete pod other-client --ignore-not-found >/dev/null
kubectl apply -f - <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: other-client
  namespace: ${NAMESPACE}
spec:
  serviceAccountName: other
  restartPolicy: OnFailure
  activeDeadlineSeconds: 180
  securityContext:
    runAsUser: 1000
    runAsGroup: 1000
    runAsNonRoot: true
  initContainers:
    - name: fetch-svid
      image: ghcr.io/spiffe/spire-agent:1.15.3
      command: ["/opt/spire/bin/spire-agent", "api", "fetch", "x509",
                "-socketPath", "/spiffe-workload-api/spire-agent.sock", "-write", "/svid"]
      volumeMounts:
        - name: spiffe-workload-api
          mountPath: /spiffe-workload-api
          readOnly: true
        - name: svid
          mountPath: /svid
  containers:
    - name: curl
      image: curlimages/curl:7.69.0
      command:
        - sh
        - -c
        - |
          grep -q . /svid/svid.0.pem || exit 1
          if curl -sk --max-time 10 --cert /svid/svid.0.pem --key /svid/svid.0.key \
              https://podinfo.${NAMESPACE}:9899/api/info; then
            echo "FAIL: unauthorized client was accepted"
            exit 2
          fi
          echo "unauthorized client was rejected"
      volumeMounts:
        - name: svid
          mountPath: /svid
          readOnly: true
  volumes:
    - name: spiffe-workload-api
      csi:
        driver: csi.spiffe.io
        readOnly: true
    - name: svid
      emptyDir: {}
EOF
kubectl -n "${NAMESPACE}" wait --for=jsonpath='{.status.phase}'=Succeeded pod/other-client --timeout=3m \
    || fail "unauthorized client check did not succeed"
kubectl -n "${NAMESPACE}" logs pod/other-client -c curl | grep -q "unauthorized client was rejected" \
    || fail "unauthorized client was not rejected"
kubectl -n "${NAMESPACE}" logs "${POD}" | jq -e --arg id "${OTHER_ID}" 'select(.spiffe_id == $id)' >/dev/null \
    && fail "podinfo served a request for ${OTHER_ID}"
info "client ${OTHER_ID} was rejected"

log "SUCCESS: all podinfo SPIFFE mTLS checks passed"
