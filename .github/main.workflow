workflow "Publish container" {
  on = "push"
  resolves = ["Docker tag and push"]
}

action "Test and build" {
  uses = "actions/docker/cli@master"
  args = "build -t app -f Dockerfile.ci ."
}

action "Docker login" {
  needs = ["Test and build"]
  uses = "actions/docker/login@master"
  secrets = ["DOCKER_USERNAME", "DOCKER_PASSWORD"]
}

action "Docker tag and push" {
  needs = ["Docker login"]
  uses = "./.github/actions/docker"
  secrets = ["DOCKER_IMAGE"]
}
