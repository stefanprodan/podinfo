#!/bin/sh
set -e

# Init step of the warm-cache job: logs the connection, request and
# response details of a call to the frontend.

FRONTEND="${FRONTEND_URL:-http://frontend}"

echo "Dumping debug data for ${FRONTEND}/api/info"
# curl -v writes to stderr; merge it into stdout to keep it in order with
# the response body.
curl -v "${FRONTEND}/api/info" 2>&1
