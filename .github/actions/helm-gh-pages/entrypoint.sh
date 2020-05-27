#!/usr/bin/env bash

set -o errexit
set -o pipefail

GITHUB_TOKEN=$1
CHARTS_DIR=$2
CHARTS_URL=$3
USER=$4
REPOSITORY=$5
BRANCH=$6

HELM_VERSION=3.2.1
CHARTS_TMP_DIR=$(mktemp -d)
REPO_ROOT=$(git rev-parse --show-toplevel)

main() {
  if [[ -z "$CHARTS_DIR" ]]; then
      CHARTS_DIR="charts"
  fi

  if [[ -z "$USER" ]]; then
      USER=$(cut -d '/' -f 1 <<< "$GITHUB_REPOSITORY")
  fi

  if [[ -z "$REPOSITORY" ]]; then
      REPOSITORY=$(cut -d '/' -f 2 <<< "$GITHUB_REPOSITORY")
  fi

  if [[ -z "$BRANCH" ]]; then
      BRANCH="gh-pages"
  fi

  if [[ -z "$CHARTS_URL" ]]; then
      CHARTS_URL="https://${USER}.github.io/${REPOSITORY}"
  fi

  download
  lint
  package
  upload
}

download() {
  tmpDir=$(mktemp -d)

  pushd $tmpDir >& /dev/null

  curl -sSL https://get.helm.sh/helm-v${HELM_VERSION}-linux-amd64.tar.gz | tar xz
  cp linux-amd64/helm /usr/local/bin/helm

  popd >& /dev/null
  rm -rf $tmpDir
}

lint() {
  helm lint ${REPO_ROOT}/${CHARTS_DIR}/*
}

package() {
  helm package ${REPO_ROOT}/${CHARTS_DIR}/* --destination ${CHARTS_TMP_DIR}
}

upload() {
  tmpDir=$(mktemp -d)
  pushd $tmpDir >& /dev/null

  repo_url="https://x-access-token:${GITHUB_TOKEN}@github.com/${USER}/${REPOSITORY}"
  git clone ${repo_url}
  cd ${REPOSITORY}
  git config user.name "$GITHUB_ACTOR"
  git config user.email "$GITHUB_ACTOR@users.noreply.github.com"
  git remote set-url origin ${repo_url}
  git checkout gh-pages

  mv -f ${CHARTS_TMP_DIR}/*.tgz .
  helm repo index . --url ${CHARTS_URL}

  git add .
  git commit -m "Publish Helm charts"
  git push origin gh-pages

  popd >& /dev/null
  rm -rf $tmpDir
}

main
