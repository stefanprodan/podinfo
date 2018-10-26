workflow "Publish container" {
  on = "push"
  resolves = ["Docker tag and push"]
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
  uses = "actions/docker/cli@master"
  args = "build -t app -f Dockerfile.ci ."
}

action "Docker login" {
  needs = ["Build"]
  uses = "actions/docker/login@master"
  secrets = ["DOCKER_USERNAME", "DOCKER_PASSWORD"]
}

action "Docker tag and push" {
  needs = ["Docker login"]
  uses = "./.github/actions/docker"
  secrets = ["DOCKER_IMAGE"]
}
