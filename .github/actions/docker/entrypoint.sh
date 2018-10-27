#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

DOCKER_TAG="latest"
if [[ "${GITHUB_REF}" == "refs/tags"* ]]; then
    DOCKER_TAG=$(echo ${GITHUB_REF} | rev | cut -d/ -f1 | rev)
else
    DOCKER_TAG=$(echo ${GITHUB_REF} | rev | cut -d/ -f1 | rev)-$(echo ${GITHUB_SHA} | head -c7)
fi

if [[ "$1" == "build" ]]; then
   docker build -t ${DOCKER_IMAGE}:${DOCKER_TAG} \
   --build-arg REPOSITORY=${GITHUB_REPOSITORY} \
   --build-arg SHA=${GITHUB_SHA} -f $2 .
   echo "Docker image tagged as ${DOCKER_IMAGE}:${DOCKER_TAG}"
fi

if [[ "$1" == "push" ]]; then
   docker push ${DOCKER_IMAGE}:${DOCKER_TAG}
   echo "Docker image pushed to ${DOCKER_IMAGE}:${DOCKER_TAG}"
fi

