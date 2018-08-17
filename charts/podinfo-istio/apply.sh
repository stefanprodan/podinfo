#!/usr/bin/env bash

#Usage: fswatch -o ./podinfo-istio/ | xargs -n1 ./podinfo-istio/apply.sh

set -e

MARK='\033[0;32m'
NC='\033[0m'

log (){
    echo -e "$(date +%Y-%m-%dT%H:%M:%S%z) ${MARK}${1}${NC}"
}

log "installing frontend"
helm upgrade frontend --install ./podinfo-istio \
  --namespace=demo \
  --set host=canary.istio.weavedx.com \
  --set gateway.name=public-gateway \
  --set gateway.create=false \
  -f ./podinfo-istio/frontend.yaml

log "installing backend"
helm upgrade backend --install ./podinfo-istio \
  --namespace=demo \
  -f ./podinfo-istio/backend.yaml

log "installing store"
helm upgrade store --install ./podinfo-istio \
  --namespace=demo \
  -f ./podinfo-istio/store.yaml

log "finished installing frontend, backend and store"


