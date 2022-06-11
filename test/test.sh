#1 /usr/bin/env sh

set -e

# wait for podinfo
kubectl rollout status deployment/podinfo --timeout=3m

# test podinfo
helm test podinfo
