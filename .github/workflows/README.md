### GitHub Actions Examples

Welcome from the GitHub Actions and Flux Demo config repo! ([Backlink][FIXME])

##### Original Credit

There are some examples here taken from Stefan Prodan's original work, that may have drifted by the time you read this. See his original work at [stefanprodan/podinfo](https://github.com/stefanprodan/podinfo).

* `cve-scan.yml`
* `e2e.yml`
* `release.yml`
* `test.yml`

These are great examples of how to use GitHub Actions, but I needed a few different examples for my demonstration.

#### Image Building Examples

* `dev.yml` - The original `release.yml` is, well, a lot! This example only shows the basics, by comparison. Docker build and push for any new commit on any branch.
* `unsigned.yml` - Inspired by `release.yml`, but slimmed down from things that I'm not prepared to explain today... here we will build and pushes images for each new tag.

The dev and unsigned builder uses the same repository; this can be improved upon significantly through Stefan's original examples.

The dev builder will build and push a timestamped image for any new commit on any branch. The unsigned release builder also uses Docker build and push.

#### Bonus Image Building Examples

If we have time... I'll also show how Cloud Native Buildpacks can be used to skip the Dockerfile!

[TODO][FIXME] IOU one CNB example for Go - Kingdon

#### Helm Chart Example

FIXME: add documentation

#### Kustomize Example

FIXME: add documentation
