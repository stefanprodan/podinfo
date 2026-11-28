# Podinfo API

## HTTP API

* `GET /` serves the web UI to browsers and prints the runtime information to other clients
* `GET /api/info` returns the runtime information as JSON, polled by the web UI
* `GET /api/pod` returns the runtime information, the environment variables and the request headers of one pod, with secret values masked, polled by the web UI while its spec sheet is open
* `GET /version` prints podinfo version and git commit hash
* `GET /metrics` returns HTTP requests duration and Go runtime metrics
* `GET /healthz` used by Kubernetes liveness probe
* `GET /readyz` used by Kubernetes readiness probe
* `POST /readyz/enable` signals the Kubernetes LB that this instance is ready to receive traffic
* `POST /readyz/disable` signals the Kubernetes LB to stop sending requests to this instance
* `POST /fault_injection/enable` makes this instance respond with HTTP 500 to all application endpoints (probes, metrics, pprof and the `/fault_injection/*` control endpoints stay healthy), for testing client-side circuit breakers and outlier detection against a single failing replica
* `POST /fault_injection/disable` restores normal responses
* `GET /fault_injection/status` returns the current fault injection state (`enabled` or `disabled`)
* `GET/POST/PUT /status/{code}` returns the status code
* `GET /panic` crashes the process with exit code 255
* `POST /echo` and `POST /api/echo` forward the call to the backend service and echo the posted content, without backends the content is logged at debug level
* `GET/POST /env` returns the environment variables as a JSON array, with secret values masked
* `GET/POST /headers` returns a JSON with the request HTTP headers, with credential values masked, plus `X-Spiffe-Id` for SPIFFE mTLS clients
* `GET /delay/{seconds}` waits for the specified period
* `POST /token` issues a JWT token valid for one minute `JWT=$(curl -sd 'anon' podinfo:9898/token | jq -r .token)`
* `GET /token/validate` validates the JWT token `curl -H "Authorization: Bearer $JWT" podinfo:9898/token/validate`
* `GET /configs` returns a JSON with configmaps and/or secrets mounted in the `config` volume
* `POST/PUT /cache/{key}` saves the posted content to Redis or Valkey
* `GET /cache/{key}` returns the content from Redis or Valkey if the key exists
* `DELETE /cache/{key}` deletes the key from Redis or Valkey if it exists
* `POST/PUT /store` writes the posted content to disk at /data/hash and returns the SHA1 hash of the content
* `GET /store/{hash}` returns the content of the file /data/hash if it exists
* `GET/HEAD /artifact` returns the files from `--artifact-path` (default `./kustomize`) as a tar.gz, with the sha256 digest in the `X-Artifact-Digest` header
* `GET /ws/echo` echos content via websockets `podcli ws ws://localhost:9898/ws/echo`
* `GET /chunked/{seconds}` uses `transfer-encoding` type `chunked` to give a partial response and then waits for the specified period
* `GET /swagger/` serves the Swagger UI
* `GET/HEAD /swagger.json` returns the Swagger 2.0 spec, used for Linkerd service profiling and Gloo routes discovery

Errors are returned with their HTTP status code and a JSON body with the `code` and the `message`.

## gRPC API

* `/grpc.health.v1.Health/Check` health checking
* `/echo.v1.EchoService/Echo` echos the received content and calls `Echo` on the `--grpc-backend-url` backends
* `/version.v1.VersionService/Version` returns podinfo version and Git commit hash
* `/cache.v1.CacheService/Set` saves the value under the key in Redis or Valkey
* `/cache.v1.CacheService/Get` returns the value of the key from Redis or Valkey, or `NOT_FOUND` if the key does not exist
* `/cache.v1.CacheService/Delete` deletes the key from Redis or Valkey
* `/delay.v1.DelayService/Delay` returns a successful response after the given seconds in the body of gRPC request
* `/env.v1.EnvService/Env` returns environment variables as a JSON array, with secret values masked
* `/header.v1.HeaderService/Header` returns the headers present in the gRPC request, with credential values masked, plus `x-spiffe-id` for SPIFFE mTLS clients. Any custom header can also be given as a part of request and that can be returned using this API
* `/info.v1.InfoService/Info` returns the runtime information
* `/pod.v1.PodService/Pod` returns the runtime information, the environment variables and the request metadata of one pod, with secret values masked, plus `x-spiffe-id` for SPIFFE mTLS clients
* `/panic.v1.PanicService/Panic` crashes the process
* `/status.v1.StatusService/Status` returns the gRPC Status code given in the request body
* `/store.v1.StoreService/Write` writes the content to disk at /data/hash and returns the SHA1 hash of the content
* `/store.v1.StoreService/Read` returns the content of the file /data/hash, or `NOT_FOUND` if it does not exist
* `/token.v1.TokenService/TokenGenerate` issues a JWT token valid for one minute
* `/token.v1.TokenService/TokenValidate` validates the JWT token

## Limits

Both APIs accept request bodies, cache values and store files up to 10 MiB, and cache keys up to 1 KiB without a slash.
An echo backend response over 64 MiB is reported as a failed backend call.
gRPC clients that read values or files over 4 MiB must raise their receive limit, for example `grpcurl -max-msg-sz 10486784`.
