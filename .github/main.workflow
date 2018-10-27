workflow "Publish container" {
  on = "push"
  resolves = ["Push"]
}

action "Lint" {
  uses = "./.github/actions/golang"
  args = "fmt"
}

action "Test" {
  needs = ["Lint"]
  uses = "./.github/actions/golang"
  args = "test"
}

action "Build" {
  needs = ["Test"]
  uses = "./.github/actions/docker"
  secrets = ["DOCKER_IMAGE"]
  args = ["build", "Dockerfile.gh"]
}

action "Login" {
  needs = ["Build"]
  uses = "actions/docker/login@master"
  secrets = ["DOCKER_USERNAME", "DOCKER_PASSWORD"]
}

action "Push" {
  needs = ["Login"]
  uses = "./.github/actions/docker"
  secrets = ["DOCKER_IMAGE"]
  args = "push"
}
