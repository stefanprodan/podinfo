# podinfo end-to-end testing

The e2e testing infrastructure is powered by CircleCI and [Kubernetes Kind](https://github.com/kubernetes-sigs/kind).

### CI workflow

* download go modules
* run unit tests
* build container
* install kubectl, Helm v3 and Kubernetes Kind CLIs
* create local Kubernetes cluster with kind
* load podinfo image onto the local cluster
* deploy podinfo with Helm
* set the podinfo image to the locally built one
* run Helm tests

```yaml
jobs:
  e2e-kubernetes:
    machine: true
    steps:
      - checkout
      - run:
          name: Build podinfo container
          command: e2e/build.sh
      - run:
          name: Start Kubernetes Kind cluster
          command: e2e/bootstrap.sh
      - run:
          name: Install podinfo with Helm
          command: e2e/install.sh
      - run:
          name: Run Helm tests
          command: e2e/test.sh
```
