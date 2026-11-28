// Package signals handles termination signals and the graceful shutdown of
// the podinfo servers.
//
// SetupSignalHandler returns a channel closed on the first SIGINT or
// SIGTERM; a second signal exits the process at once. Shutdown.Graceful then
// marks the pod unhealthy and not ready, waits for the drain delay so that
// load balancers stop sending traffic, and shuts the HTTP, HTTPS and gRPC
// servers down in parallel, forcing the gRPC server to stop when the timeout
// expires.
package signals
