#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

echo "Starting tag and push for image ${DOCKER_IMAGE} release ${GITHUB_REF}"

DOCKER_TAG="latest"
if [[ "${GITHUB_REF}" == "refs/tags"* ]]; then
    DOCKER_TAG=$(echo ${GITHUB_REF} | rev | cut -d/ -f1 | rev)
else
    DOCKER_TAG=$(echo ${GITHUB_REF} | rev | cut -d/ -f1 | rev)-$(echo ${GITHUB_SHA} | head -c7)
fi

docker tag app ${DOCKER_IMAGE}:${DOCKER_TAG}
docker push ${DOCKER_IMAGE}:${DOCKER_TAG}

echo "Docker image pushed to ${DOCKER_IMAGE}:${DOCKER_TAG}"
