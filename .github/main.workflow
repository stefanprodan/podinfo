workflow "Publish branch" {
  on = "push"
  resolves = ["Is branch", "Push branch"]
}

action "Is branch" {
  uses = "actions/bin/filter@master"
  args = "ref refs/heads/*"
}

action "Test and build branch" {
  needs = ["Is branch"]
  uses = "actions/docker/cli@master"
  args = "build -t app -f Dockerfile.ci ."
}

action "Tag branch" {
  needs = ["Test and build branch"]
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


workflow "Publish release" {
  on = "release"
  resolves = ["Push release"]
}

action "Is release" {
  uses = "actions/bin/filter@master"
  args = "ref refs/tags/v*"
}

action "Test and build release" {
  needs = ["Is release"]
  uses = "actions/docker/cli@master"
  args = "build -t app -f Dockerfile.ci ."
}

action "Login release" {
  needs = ["Test and build release"]
  uses = "actions/docker/login@master"
  secrets = ["DOCKER_USERNAME", "DOCKER_PASSWORD"]
}

action "Tag release" {
  needs = ["Login release"]
  uses = "actions/docker/cli@master"
  args = "tag app ${DOCKER_IMAGE}:$(echo ${GITHUB_REF} | rev | cut -d/ -f1 | rev)"
  secrets = ["DOCKER_IMAGE"]
}

action "Push release" {
  needs = ["Tag release"]
  uses = "actions/docker/cli@master"
  args = "push ${DOCKER_IMAGE}:$(echo ${GITHUB_REF} | rev | cut -d/ -f1 | rev)"
  secrets = ["DOCKER_IMAGE"]
}
