# Podinfo Cache

The `/cache/{key}` endpoints and the gRPC `cache.v1.CacheService` share one client that stores values in Redis or Valkey:

```bash
podinfo --cache-server=redis://redis:6379
```

| Setting                                                     | Description                                                                                         |
|-------------------------------------------------------------|-----------------------------------------------------------------------------------------------------|
| `--cache-server`                                            | `redis://[user:password@]host:port[/db]`, `rediss://` for TLS, `valkey://` and `valkeys://` as aliases |
| `--cache-cluster`                                           | Cluster mode, implied when the URL has more addresses as `?addr=host:port` parameters               |
| `--cache-sentinel-primary`                                  | Name of the primary monitored by Sentinel, the URL addresses are then the Sentinels                 |
| `--cache-ca-file`                                           | CA bundle that verifies the server certificate instead of the system roots                          |
| `--cache-cert-path`                                         | Directory with the `tls.crt` and `tls.key` client key pair for mTLS, reloaded on rotation           |
| `PODINFO_CACHE_USERNAME`, `PODINFO_CACHE_PASSWORD`          | Credentials, instead of putting them in the URL                                                     |
| `PODINFO_CACHE_SENTINEL_PASSWORD`                           | Password of the Sentinels                                                                           |

Without `--cache-server` the endpoints return HTTP 503 and the gRPC service returns `UNAVAILABLE`. An invalid cache configuration is a startup error.

## Cluster and Sentinel with TLS

The client dials the addresses that the cluster nodes and Sentinels announce, not the hostnames in the URL.
With TLS, the servers must announce hostnames instead of pod IPs, or the certificate verification fails:

- Cluster: `cluster-announce-hostname <hostname>` and `cluster-preferred-endpoint-type hostname`
- Sentinel: `replica-announce-ip <hostname>` on the servers, `sentinel resolve-hostnames yes` and `sentinel announce-hostnames yes` on the Sentinels
