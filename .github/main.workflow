workflow "Publish branch" {
  on = "push"
  resolves = ["Is branch", "Push branch"]
}

action "Is branch" {
  uses = "actions/bin/filter@master"
  args = "ref refs/heads/*"
}

action "Test branch" {
  needs = ["Is branch"]
  uses = "docker://golang:1.10"
  runs = "sh -c"
  args = ["cp -a vendor/. /usr/local/go/src; go test -v ./..."]
}

action "Build branch" {
  needs = ["Test branch"]
  uses = "actions/docker/cli@master"
  args = "build -t app -f Dockerfile.ci ."
}

action "Tag branch" {
  needs = ["Build branch"]
  uses = "actions/docker/cli@master"
  args = "tag app ${DOCKER_IMAGE}:$(echo ${GITHUB_REF} | rev | cut -d/ -f1 | rev)-$(echo ${GITHUB_SHA} | head -c7)"
  secrets = ["DOCKER_IMAGE"]
}

action "Login branch" {
  needs = ["Tag branch"]
  uses = "actions/docker/login@master"
  secrets = ["DOCKER_USERNAME", "DOCKER_PASSWORD"]
}

action "Push branch" {
  needs = ["Login branch"]
  uses = "actions/docker/cli@master"
  args = "push ${DOCKER_IMAGE}:$(echo ${GITHUB_REF} | rev | cut -d/ -f1 | rev)-$(echo ${GITHUB_SHA} | head -c7)"
  secrets = ["DOCKER_IMAGE"]
}


