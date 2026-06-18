#!/bin/sh
set -e

# Init step for the warm-cache job.
# Calls the frontend in verbose mode so the full connection, request and
# response debug data (DNS, TCP, headers, timings) is dumped to the logs.

FRONTEND="${FRONTEND_URL:-http://frontend}"

echo "Dumping debug data for ${FRONTEND}/api/info"
# -v writes the verbose debug data to stderr, redirect it to stdout so it
# lands in the container logs together with the response body.
curl -v "${FRONTEND}/api/info" 2>&1
