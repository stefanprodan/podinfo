#!/bin/sh
set -e

# This is a simulation of a cache warming process.
# It fetches the frontend info, stores it in the cache under the job pod
# hostname and reads it back to verify the round-trip.
#
# Log lines mimic the Log4j 2 default PatternLayout:
#   %d{yyyy-MM-dd HH:mm:ss,SSS} [%thread] %-5level %logger{36} - %msg

LOGGER="com.stefanprodan.podinfo.WarmCache"
THREAD="main"

log() {
  level="$1"
  shift
  # millisecond precision timestamp; fall back gracefully if %N is unsupported
  ts="$(date '+%Y-%m-%d %H:%M:%S,%N' 2>/dev/null)"
  case "$ts" in
    *%N|*,) ts="$(date '+%Y-%m-%d %H:%M:%S'),000" ;;
    *) ts="$(echo "$ts" | cut -c1-23)" ;;
  esac
  printf '%s [%s] %-5s %s - %s\n' "$ts" "$THREAD" "$level" "$LOGGER" "$*"
}

FRONTEND="${FRONTEND_URL:-http://frontend}"
KEY="$(hostname)"
START="$(date +%s)"

log INFO "Fetching info from ${FRONTEND}/api/info"
# Compact the (pretty-printed) JSON onto a single line so each log event
# stays on one line, as Log4j renders them.
INFO="$(curl -fsS "${FRONTEND}/api/info" | tr -d '\n' | sed 's/  */ /g')"
log INFO "Info: ${INFO}"

# Log each field of the info response on its own line. The /api/info payload
# is a flat JSON object of string values, so we can split it without jq.
echo "$INFO" | sed 's/^{//; s/}$//' | tr ',' '\n' | while IFS= read -r field; do
  field="$(echo "$field" | sed 's/^[[:space:]]*//; s/[[:space:]]*$//; s/"//g')"
  if [ -n "$field" ]; then
    log DEBUG "info ${field}"
  fi
done

log INFO "Writing info to cache key ${KEY}"
curl -fsS -X POST -H "Content-Type: application/json" -d "${INFO}" "${FRONTEND}/cache/${KEY}"

# Verify the cached value for 1 minute, polling every 10 seconds.
attempt=1
while [ "$attempt" -le 6 ]; do
  log INFO "Reading cache key ${KEY} (attempt ${attempt}/6)"
  CACHED="$(curl -fsS "${FRONTEND}/cache/${KEY}")"

  if [ "${INFO}" = "${CACHED}" ]; then
    log INFO "Cache warm verified: cached value matches info"
  else
    log ERROR "Cache warm failed: cached value does not match info"
    log ERROR "Expected: ${INFO}"
    log ERROR "Got:      ${CACHED}"
    exit 1
  fi

  attempt=$((attempt + 1))
  if [ "$attempt" -le 6 ]; then
    sleep 10
  fi
done

log INFO "Cache warm finished in $(( $(date +%s) - START ))s"
