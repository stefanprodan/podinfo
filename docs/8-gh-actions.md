# GitHub Actions

Create a private repository named `demo-app` on GitHub and navigate to Settings/Secrets and add the following secrets:

* `DOCKER_IMAGE` eg stefanprodan/demo-app
* `DOCKER_USERNAME` eg stefanprodan
* `DOCKER_PASSWORD` eg my-docker-hub-pass

Install podinfo CLI:

```bash
brew install weaveworks/tap/podcli
```

For linux or Windows go to the 
[release page](https://github.com/stefanprodan/k8s-podinfo/releases), download the latest podcli release and add it to your path.

Clone your private repository (preferable in your `$GOPATH`) and initialize podinfo.

```bash
git clone https://github.com/stefanprodan/demo-app
cd demo-app

podcli code init demo-app --git-user=stefanprodan --version=v1.3.1
```

The above command does the following:
* downloads podinfo source code v1.3.1 from GitHub 
* replaces golang imports with your git username and project name
* creates a Dockerfile and Makefile customized for GitHub actions
* creates the main workflow for GitHub actions
* commits and pushes the code to GitHub

When the code init command finishes, GitHub will test, build and push a Docker image 
`${DOCKER_IMAGE}:${GIT-BRANCH}-${GIT-SHORT-SHA}` to your Docker Hub account.

If you create a GitHub release a Docker image with the format `${DOCKER_IMAGE}:${GIT-TAG}` will be published to Docker Hub.

![github-actions-ci](https://github.com/stefanprodan/k8s-podinfo/blob/master/docs/screens/github-actions-ci.png)

