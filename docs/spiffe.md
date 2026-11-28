# Podinfo SPIFFE mTLS

Podinfo can serve its HTTPS port with [SPIFFE](https://spiffe.io) mTLS. The server gets its X.509 SVID
from the SPIFFE Workload API (e.g. a [SPIRE](https://spiffe.io/docs/latest/spire-about/) agent),
rotates it automatically, and only accepts clients that present an SVID from the same trust bundle.

| Flag                       | Description                                                                                                                                 |
|----------------------------|---------------------------------------------------------------------------------------------------------------------------------------------|
| `--spiffe-endpoint-socket` | Workload API address, e.g. `unix:///spiffe-workload-api/spire-agent.sock`, requires `--secure-port`                                         |
| `--spiffe-authorized-ids`  | Client SPIFFE IDs allowed to connect, when empty any ID in podinfo's trust domain is allowed                                                |
| `--spiffe-backend-mtls`    | Call the `https` and `grpcs` backends with SPIFFE mTLS, requires `--spiffe-endpoint-socket`                                                 |
| `--spiffe-backend-ids`     | Server SPIFFE IDs accepted from the backends, when empty any ID in podinfo's trust domain is accepted, requires `--spiffe-backend-mtls`     |

Podinfo starts the HTTP port and passes the liveness probe right away, and keeps retrying the Workload API
until the SVID is issued. The readiness probe fails until the HTTPS port is serving with the SVID.
The request log includes the `spiffe_id` of the client at debug level.

With `--grpc-tls`, the gRPC port is served with the same SVID and authorized IDs, and the gRPC health
of the `--grpc-service-name` service is `NOT_SERVING` until the SVID is issued.
Without SPIFFE, `--grpc-tls` serves the gRPC port with the `--cert-path` certificate.

With `--spiffe-backend-mtls`, `/api/echo` calls the `https://` backends set with `--backend-url` over SPIFFE mTLS,
presenting the same SVID and verifying the backend against `--spiffe-backend-ids`.
All `https://` backends must then be SPIFFE workloads, calls to `http://` backends are unchanged.
Without the flag, backends are called with the default TLS settings. To chain two podinfo instances,
point the frontend at the backend's HTTPS port and authorize the frontend's ID on the backend:

```shell
# backend
podinfo --secure-port=9899 --spiffe-endpoint-socket=unix:///spiffe-workload-api/spire-agent.sock \
  --spiffe-authorized-ids=spiffe://example.org/ns/demo/sa/frontend

# frontend
podinfo --secure-port=9899 --spiffe-endpoint-socket=unix:///spiffe-workload-api/spire-agent.sock \
  --backend-url=https://backend:9899/api/echo \
  --spiffe-backend-mtls \
  --spiffe-backend-ids=spiffe://example.org/ns/demo/sa/backend
```

The gRPC `Echo` service calls the backends set with `--grpc-backend-url`, in plaintext for `grpc://host:port`
and with TLS verified against the system roots for `grpcs://host:port`. With `--spiffe-backend-mtls`, the `grpcs://`
backends are called over SPIFFE mTLS like the `https://` ones and must serve gRPC with `--grpc-tls`.
`--grpc-client-timeout` limits the duration of a backend call.

On a cluster with SPIRE and the [SPIFFE CSI driver](https://github.com/spiffe/spiffe-csi),
register the podinfo pods with the SPIRE controller manager:

```yaml
apiVersion: spire.spiffe.io/v1alpha1
kind: ClusterSPIFFEID
metadata:
  name: podinfo
spec:
  # the SPIRE controller manager class, for the SPIRE Helm chart: <namespace>-<release>
  className: spire-system-spire
  spiffeIDTemplate: "spiffe://{{ .TrustDomain }}/ns/{{ .PodMeta.Namespace }}/sa/{{ .PodSpec.ServiceAccountName }}"
  podSelector: {}
  namespaceSelector:
    matchLabels:
      kubernetes.io/metadata.name: podinfo
```

Install with Helm:

```bash
helm upgrade --install --wait podinfo --namespace podinfo \
--set serviceAccount.enabled=true \
--set tls.enabled=true \
--set spiffe.enabled=true \
--set spiffe.authorizedIDs={spiffe://example.org/ns/podinfo/sa/podinfo} \
oci://ghcr.io/stefanprodan/charts/podinfo
```

The Helm test client runs with podinfo's service account, so its ID is included in `authorizedIDs` above.
