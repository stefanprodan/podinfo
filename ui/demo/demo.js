// Fake API for reviewing the UI with `make run-ui-demo`; not embedded in the binary.
// Plays a Flagger canary release from 6.17.0, which predates /api/pod and answers it
// with 404, to 7.0.0: the canary pods take a growing share of traffic up to half, the
// primary is promoted to the new version, then the API goes away for a few seconds.
// Every fourth ping fails.

let step = 0,
  pings = 0;

function info() {
  step++;
  if (step > 46 && step <= 50) throw new TypeError("demo outage");
  const promoted = step > 36;
  const canary =
    !promoted && Math.random() < Math.max(0, Math.min(0.5, (step - 4) / 60));
  const pods = promoted
    ? [
        "podinfo-primary-5d8c7f6b94-m2xvd",
        "podinfo-primary-5d8c7f6b94-t9bqk",
        "podinfo-primary-5d8c7f6b94-w6rhn",
      ]
    : canary
      ? ["podinfo-6f8d7c9b54-h7xkq", "podinfo-6f8d7c9b54-r2m9d"]
      : [
          "podinfo-primary-7b5c6d8f9-4tzwn",
          "podinfo-primary-7b5c6d8f9-kq8sv",
          "podinfo-primary-7b5c6d8f9-zl2pj",
        ];
  const next = promoted || canary;
  return {
    hostname: pods[Math.floor(Math.random() * pods.length)],
    version: next ? "7.0.0" : "6.17.0",
    revision: next
      ? "59437a333ca331d7f50947a03d389dc26e7a5123"
      : "1c192f7a8e6990b4d2f0c3e5a7b9d1f3e5a7c9b1",
    color: next ? "#1f4e8c" : "#34577c",
    message: "greetings from podinfo",
    logo: "/ui/cuddle_clap.gif",
    goos: "linux",
    goarch: "amd64",
    runtime: next ? "go1.27.4" : "go1.26.2",
    num_goroutine: String(7 + Math.floor(Math.random() * 4)),
    num_cpu: "4",
  };
}

function json(body, status) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

function env(hostname, color) {
  return {
    PATH: "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
    HOSTNAME: hostname,
    PODINFO_UI_COLOR: color,
    PODINFO_CACHE_SERVER: "rediss://podinfo-redis:6379",
    PODINFO_CACHE_PASSWORD: "[REDACTED]",
    OTEL_EXPORTER_OTLP_ENDPOINT: "http://otel-collector.monitoring:4318",
    OTEL_RESOURCE_ATTRIBUTES:
      "k8s.namespace.name=apps,k8s.pod.name=" +
      hostname +
      ",k8s.node.name=kind-worker",
    OTEL_SERVICE_NAME: "podinfo",
    KUBERNETES_SERVICE_HOST: "10.96.0.1",
    KUBERNETES_SERVICE_PORT: "443",
  };
}

const headers = {
  Accept: ["*/*"],
  Cookie: ["[REDACTED]"],
  Traceparent: ["00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"],
  "User-Agent": [navigator.userAgent],
  "X-Forwarded-For": ["203.0.113.24, 10.244.0.1"],
  "X-Forwarded-Host": ["podinfo.example.com"],
  "X-Forwarded-Proto": ["https"],
  "X-Request-Id": ["3f9c1b2e-7a4d-4c8e-9b1f-2d6e8a0c4f5b"],
};

function pod() {
  const { color, logo, message, ...runtime } = info();
  if (runtime.version.startsWith("6."))
    return new Response("404 page not found", { status: 404 });
  return json({ ...runtime, env: env(runtime.hostname, color), headers }, 200);
}

window.fetch = (url) => {
  if (String(url).endsWith("/api/info"))
    return new Promise((resolve) => resolve(json(info(), 200)));
  if (String(url).endsWith("/api/pod"))
    return new Promise((resolve) => resolve(pod()));
  return new Promise((resolve, reject) => {
    const fail = ++pings % 4 === 0;
    setTimeout(
      () => {
        if (fail) reject(new TypeError("demo failure"));
        else resolve(json({}, 202));
      },
      200 + Math.random() * 900,
    );
  });
};
