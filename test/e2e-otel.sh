#!/usr/bin/env bash
# End-to-end test for podinfo OpenTelemetry export on a kind cluster.
set -euo pipefail

OTEL_NAMESPACE="otel"
HELM_NAMESPACE="podinfo"
TIMONI_NAMESPACE="podinfo-timoni"
COLLECTOR_VERSION="${COLLECTOR_VERSION:-0.166.0}"
GRPCURL_VERSION="${GRPCURL_VERSION:-v1.9.3}"
CHECK_TIMEOUT="${CHECK_TIMEOUT:-180}"
TMP_DIR="$(mktemp -d)"
RECORDS="${TMP_DIR}/records.tsv"

log()  { printf '\n\033[1;34m==> %s\033[0m\n' "$*"; }
info() { printf '    %s\n' "$*"; }
fail() { printf '\033[1;31mFAIL: %s\033[0m\n' "$*" >&2; exit 1; }

cleanup() {
    local code=$?
    if [[ $code -ne 0 ]]; then
        log "FAILURE (exit ${code}) - dumping diagnostics"
        for ns in "${OTEL_NAMESPACE}" "${HELM_NAMESPACE}" "${TIMONI_NAMESPACE}"; do
            kubectl -n "${ns}" get pods -o wide || true
            kubectl -n "${ns}" describe pods || true
        done
        for ns in "${HELM_NAMESPACE}" "${TIMONI_NAMESPACE}"; do
            kubectl -n "${ns}" logs -l 'app.kubernetes.io/name' --all-containers --tail=50 || true
        done
        kubectl -n "${OTEL_NAMESPACE}" logs deploy/otel-collector --tail=200 || true
        awk -F'\t' '{print $1, $2}' "${RECORDS}" 2>/dev/null | sort | uniq -c || true
    fi
    rm -rf "${TMP_DIR}"
    exit "$code"
}
trap cleanup EXIT

for bin in kubectl helm timoni kind docker awk; do
    command -v "${bin}" >/dev/null 2>&1 || fail "required tool '${bin}' not found"
done

random_hex() { od -An -N"$1" -tx1 /dev/urandom | tr -d ' \n'; }

log "Building podinfo"
docker build --tag test/podinfo --build-arg "REVISION=0.0.0-$(git rev-list -1 HEAD)" .
kind load docker-image test/podinfo:latest

log "Installing the OpenTelemetry Collector"
kubectl create namespace "${OTEL_NAMESPACE}" --dry-run=client -o yaml | kubectl apply -f - >/dev/null
kubectl apply -f - <<EOF
apiVersion: v1
kind: ConfigMap
metadata:
  name: otel-collector
  namespace: ${OTEL_NAMESPACE}
data:
  config.yaml: |
    receivers:
      otlp:
        protocols:
          grpc:
            endpoint: 0.0.0.0:4317
          http:
            endpoint: 0.0.0.0:4318
    exporters:
      debug:
        verbosity: detailed
    service:
      pipelines:
        traces:
          receivers: [otlp]
          exporters: [debug]
        metrics:
          receivers: [otlp]
          exporters: [debug]
        logs:
          receivers: [otlp]
          exporters: [debug]
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: otel-collector
  namespace: ${OTEL_NAMESPACE}
spec:
  replicas: 1
  selector:
    matchLabels:
      app: otel-collector
  template:
    metadata:
      labels:
        app: otel-collector
    spec:
      containers:
        - name: otel-collector
          image: otel/opentelemetry-collector:${COLLECTOR_VERSION}
          args: ["--config=/conf/config.yaml"]
          ports:
            - name: otlp-grpc
              containerPort: 4317
            - name: otlp-http
              containerPort: 4318
          volumeMounts:
            - name: config
              mountPath: /conf
      volumes:
        - name: config
          configMap:
            name: otel-collector
---
apiVersion: v1
kind: Service
metadata:
  name: otel-collector
  namespace: ${OTEL_NAMESPACE}
spec:
  selector:
    app: otel-collector
  ports:
    - name: otlp-grpc
      port: 4317
      appProtocol: grpc
    - name: otlp-http
      port: 4318
      appProtocol: http
EOF
kubectl -n "${OTEL_NAMESPACE}" rollout status deployment/otel-collector --timeout=3m

log "Installing podinfo backend and frontend with Helm"
kubectl create namespace "${HELM_NAMESPACE}" --dry-run=client -o yaml | kubectl apply -f - >/dev/null
# The backend exports over gRPC and the frontend over the default http/protobuf.
helm upgrade --install backend ./charts/podinfo \
    --namespace "${HELM_NAMESPACE}" \
    --set image.repository=test/podinfo \
    --set image.tag=latest \
    --set logLevel=debug \
    --set otel.enabled=true \
    --set otel.endpoint="http://otel-collector.${OTEL_NAMESPACE}:4317" \
    --set otel.protocol=grpc \
    --set otel.resourceAttributes.deployment\\.environment\\.name=e2e \
    --set 'extraEnvs[0].name=OTEL_METRIC_EXPORT_INTERVAL' \
    --set-string 'extraEnvs[0].value=5000' \
    --wait --timeout 3m
helm upgrade --install frontend ./charts/podinfo \
    --namespace "${HELM_NAMESPACE}" \
    --set image.repository=test/podinfo \
    --set image.tag=latest \
    --set backend="http://backend-podinfo:9898/echo" \
    --set 'grpcBackends={grpc://backend-podinfo:9999}' \
    --set logLevel=debug \
    --set otel.enabled=true \
    --set otel.endpoint="http://otel-collector.${OTEL_NAMESPACE}:4318" \
    --set otel.resourceAttributes.deployment\\.environment\\.name=e2e \
    --set 'extraEnvs[0].name=OTEL_METRIC_EXPORT_INTERVAL' \
    --set-string 'extraEnvs[0].value=5000' \
    --wait --timeout 3m

log "Installing podinfo backend and frontend with Timoni"
kubectl create namespace "${TIMONI_NAMESPACE}" --dry-run=client -o yaml | kubectl apply -f - >/dev/null
cat > "${TMP_DIR}/values.cue" <<EOF
values: {
	image: {
		repository: "test/podinfo"
		tag:        "latest"
		digest:     ""
	}
	logLevel: "debug"
	otel: {
		enabled:  true
		endpoint: "http://otel-collector.${OTEL_NAMESPACE}:4318"
		resourceAttributes: "deployment.environment.name": "e2e"
	}
	env: [{name: "OTEL_METRIC_EXPORT_INTERVAL", value: "5000"}]
}
EOF
cat > "${TMP_DIR}/frontend.cue" <<EOF
values: ui: {
	backend:     "http://backend.${TIMONI_NAMESPACE}/echo"
	grpcBackend: "grpc://backend.${TIMONI_NAMESPACE}:9999"
}
EOF
timoni -n "${TIMONI_NAMESPACE}" apply backend ./timoni/podinfo -f "${TMP_DIR}/values.cue" --timeout 3m
timoni -n "${TIMONI_NAMESPACE}" apply frontend ./timoni/podinfo -f "${TMP_DIR}/values.cue" -f "${TMP_DIR}/frontend.cue" --timeout 3m

# run_client <namespace> <name> <image> <args...> runs a pod to completion.
run_client() {
    local ns=$1 name=$2 image=$3
    shift 3
    kubectl -n "${ns}" delete pod "${name}" --ignore-not-found >/dev/null
    kubectl -n "${ns}" run "${name}" --image="${image}" --image-pull-policy=IfNotPresent --restart=Never -- "$@" >/dev/null
    kubectl -n "${ns}" wait --for=jsonpath='{.status.phase}'=Succeeded "pod/${name}" --timeout=2m \
        || fail "client ${name} did not succeed in ${ns}"
    kubectl -n "${ns}" logs "pod/${name}"
}

# send_requests <namespace> <http address> <grpc address> <http trace ID> <grpc trace ID>
send_requests() {
    local ns=$1 http_addr=$2 grpc_addr=$3 http_trace=$4 grpc_trace=$5
    run_client "${ns}" http-client test/podinfo:latest \
        curl -sf -H "traceparent: 00-${http_trace}-$(random_hex 8)-01" -d otel "http://${http_addr}/echo" >/dev/null
    run_client "${ns}" grpc-client "fullstorydev/grpcurl:${GRPCURL_VERSION}" \
        -plaintext -H "traceparent: 00-${grpc_trace}-$(random_hex 8)-01" -d '{"body":"otel"}' \
        "${grpc_addr}" echo.v1.EchoService/Echo >/dev/null
}

HELM_HTTP_TRACE="$(random_hex 16)"
HELM_GRPC_TRACE="$(random_hex 16)"
TIMONI_HTTP_TRACE="$(random_hex 16)"
TIMONI_GRPC_TRACE="$(random_hex 16)"

log "Sending requests with known trace IDs"
send_requests "${HELM_NAMESPACE}" frontend-podinfo:9898 frontend-podinfo:9999 "${HELM_HTTP_TRACE}" "${HELM_GRPC_TRACE}"
info "Helm: HTTP trace ${HELM_HTTP_TRACE}, gRPC trace ${HELM_GRPC_TRACE}"
send_requests "${TIMONI_NAMESPACE}" frontend:80 frontend:9999 "${TIMONI_HTTP_TRACE}" "${TIMONI_GRPC_TRACE}"
info "Timoni: HTTP trace ${TIMONI_HTTP_TRACE}, gRPC trace ${TIMONI_GRPC_TRACE}"

# collect parses the debug exporter output into tab-separated records:
#   attr   <service> <key> <value>
#   span   <service> <trace ID> <span ID> <parent ID> <kind> <name>
#   metric <service> <name>
#   log    <service> <trace ID>
collect() {
    kubectl -n "${OTEL_NAMESPACE}" logs deploy/otel-collector --tail=-1 | awk '
        function val(s) { sub(/^[^:]*: */, "", s); return s }
        /Resource(Spans|Metrics|Log) #/ {
            sig = $0; sub(/.*Resource/, "", sig); sub(/ #.*/, "", sig)
            inres = 1; delete attrs; next
        }
        inres && /^ +-> / {
            line = $0; sub(/^ +-> /, "", line)
            key = line; sub(/: .*/, "", key)
            value = val(line); sub(/^[A-Za-z]+\(/, "", value); sub(/\)$/, "", value)
            attrs[key] = value; next
        }
        /^Scope(Spans|Metrics|Logs) #/ && inres {
            svc = attrs["service.name"]; inres = 0
            for (key in attrs) printf "attr\t%s\t%s\t%s\n", svc, key, attrs[key]
        }
        sig == "Spans" && /^ +Trace ID +:/  { trace = val($0) }
        sig == "Spans" && /^ +Parent ID +:/ { parent = val($0) }
        sig == "Spans" && /^ +ID +:/        { id = val($0) }
        sig == "Spans" && /^ +Name +:/      { name = val($0) }
        sig == "Spans" && /^ +Kind +:/      { printf "span\t%s\t%s\t%s\t%s\t%s\t%s\n", svc, trace, id, parent, val($0), name }
        sig == "Metrics" && /^ +-> Name: /  { printf "metric\t%s\t%s\n", svc, val($0) }
        sig == "Log" && /^ *Trace ID *:/    { printf "log\t%s\t%s\n", svc, val($0) }
    ' > "${RECORDS}"
}

# has <awk condition> is true when a record matches the condition.
has() { awk -F'\t' "$1 { found = 1; exit } END { exit !found }" "${RECORDS}"; }

# check <namespace> <frontend> <backend> <http trace ID> <grpc trace ID> prints the first failed assertion.
check() {
    local ns=$1 fe=$2 be=$3 http_trace=$4 grpc_trace=$5 svc deploy pod node trace client_ids
    for trace in "${http_trace}" "${grpc_trace}"; do
        has "\$1==\"span\" && \$2==\"${fe}\" && \$3==\"${trace}\" && \$6==\"Server\"" \
            || { echo "no ${fe} server span in trace ${trace}"; return 1; }
        has "\$1==\"span\" && \$2==\"${fe}\" && \$3==\"${trace}\" && \$6==\"Client\"" \
            || { echo "no ${fe} client span in trace ${trace}"; return 1; }
        has "\$1==\"span\" && \$2==\"${be}\" && \$3==\"${trace}\" && \$6==\"Server\"" \
            || { echo "no ${be} server span in trace ${trace}"; return 1; }
        client_ids=$(awk -F'\t' -v s="${fe}" -v t="${trace}" '$1=="span" && $2==s && $3==t && $6=="Client" {print $4}' "${RECORDS}" | paste -sd'|' -)
        has "\$1==\"span\" && \$2==\"${be}\" && \$3==\"${trace}\" && \$6==\"Server\" && \$5 ~ /^(${client_ids})\$/" \
            || { echo "the ${be} server span is not a child of the ${fe} client span in trace ${trace}"; return 1; }
    done
    for svc in "${fe}" "${be}"; do
        has "\$1==\"span\" && \$2==\"${svc}\" && \$3==\"${grpc_trace}\" && \$6==\"Server\" && \$7==\"echo.v1.EchoService/Echo\"" \
            || { echo "no ${svc} gRPC server span in trace ${grpc_trace}"; return 1; }
    done
    for svc in "${fe}" "${be}"; do
        has "\$1==\"metric\" && \$2==\"${svc}\" && \$3==\"http.server.request.duration\"" \
            || { echo "no http.server.request.duration metric from ${svc}"; return 1; }
        has "\$1==\"log\" && \$2==\"${svc}\"" \
            || { echo "no log records from ${svc}"; return 1; }
        has "\$1==\"log\" && \$2==\"${svc}\" && \$3==\"${http_trace}\"" \
            || { echo "no log record from ${svc} in trace ${http_trace}"; return 1; }
    done
    has "\$1==\"metric\" && \$2==\"${fe}\" && \$3==\"rpc.server.call.duration\"" \
        || { echo "no rpc.server.call.duration metric from ${fe}"; return 1; }
    for svc in "${fe}" "${be}"; do
        deploy=$(kubectl -n "${ns}" get deploy -l "app.kubernetes.io/name=${svc}" -o jsonpath='{.items[0].metadata.name}')
        pod=$(kubectl -n "${ns}" get pod -l "app.kubernetes.io/name=${svc}" -o jsonpath='{.items[0].metadata.name}')
        node=$(kubectl -n "${ns}" get pod "${pod}" -o jsonpath='{.spec.nodeName}')
        for attr in "k8s.namespace.name=${ns}" "service.namespace=${ns}" "k8s.deployment.name=${deploy}" \
            "k8s.pod.name=${pod}" "k8s.node.name=${node}" "deployment.environment.name=e2e"; do
            has "\$1==\"attr\" && \$2==\"${svc}\" && \$3==\"${attr%%=*}\" && \$4==\"${attr#*=}\"" \
                || { echo "resource attribute ${attr} missing on ${svc}"; return 1; }
        done
        has "\$1==\"attr\" && \$2==\"${svc}\" && \$3==\"k8s.pod.uid\" && length(\$4)==36" \
            || { echo "resource attribute k8s.pod.uid missing on ${svc}"; return 1; }
    done
}

log "Checking the telemetry received by the collector"
deadline=$((SECONDS + CHECK_TIMEOUT))
while true; do
    collect
    helm_result="" timoni_result=""
    helm_result=$(check "${HELM_NAMESPACE}" frontend-podinfo backend-podinfo "${HELM_HTTP_TRACE}" "${HELM_GRPC_TRACE}") \
        && timoni_result=$(check "${TIMONI_NAMESPACE}" frontend backend "${TIMONI_HTTP_TRACE}" "${TIMONI_GRPC_TRACE}") \
        && break
    if (( SECONDS >= deadline )); then
        [[ -n "${helm_result}" ]] && fail "Helm: ${helm_result}"
        fail "Timoni: ${timoni_result}"
    fi
    sleep 5
done
info "HTTP and gRPC spans from both services share the trace, with the backend server span under the frontend client span"
info "HTTP and gRPC metrics and log records with the trace ID arrived from both installs"
info "the resources carry the service names and the Kubernetes attributes"

log "SUCCESS: all podinfo OpenTelemetry checks passed"
