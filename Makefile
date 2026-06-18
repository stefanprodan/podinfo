# Makefile for releasing podinfo
#
# The release version is controlled from pkg/version

TAG?=latest
NAME:=podinfo
DOCKER_REPOSITORY:=stefanprodan
DOCKER_IMAGE_NAME:=$(DOCKER_REPOSITORY)/$(NAME)
GIT_COMMIT:=$(shell git describe --dirty --always)
VERSION:=$(shell grep 'VERSION' pkg/version/version.go | awk '{ print $$4 }' | tr -d '"')
EXTRA_RUN_ARGS?=

run:
	go run -ldflags "-s -w -X github.com/stefanprodan/podinfo/pkg/version.REVISION=$(GIT_COMMIT)" cmd/podinfo/* \
	--level=debug --grpc-port=9999 --backend-url=https://httpbin.org/status/401 --backend-url=https://httpbin.org/status/500 \
	--ui-logo=https://raw.githubusercontent.com/stefanprodan/podinfo/gh-pages/cuddle_clap.gif $(EXTRA_RUN_ARGS)

.PHONY: test
test: tidy fmt vet
	go test ./... -coverprofile cover.out

build:
	GIT_COMMIT=$$(git rev-list -1 HEAD) && CGO_ENABLED=0 go build  -ldflags "-s -w -X github.com/stefanprodan/podinfo/pkg/version.REVISION=$(GIT_COMMIT)" -a -o ./bin/podinfo ./cmd/podinfo/*
	GIT_COMMIT=$$(git rev-list -1 HEAD) && CGO_ENABLED=0 go build  -ldflags "-s -w -X github.com/stefanprodan/podinfo/pkg/version.REVISION=$(GIT_COMMIT)" -a -o ./bin/podcli ./cmd/podcli/*

tidy:
	rm -f go.sum; go mod tidy -compat=1.26

vet:
	go vet ./...

fmt:
	go fmt ./...

build-charts:
	helm lint charts/*
	helm package charts/*

build-container:
	docker build -t $(DOCKER_IMAGE_NAME):$(VERSION) .

test-container:
	@docker rm -f podinfo || true
	@docker run -dp 9898:9898 --name=podinfo $(DOCKER_IMAGE_NAME):$(VERSION)
	@docker ps
	@TOKEN=$$(curl -sd 'test' localhost:9898/token | jq -r .token) && \
	curl -sH "Authorization: Bearer $${TOKEN}" localhost:9898/token/validate | grep test

version-set:
	@next="$(TAG)" && \
	current="$(VERSION)" && \
	perl -i -pe "s/\Q$$current\E/$$next/g" pkg/version/version.go && \
	perl -i -pe "s/tag: \Q$$current\E/tag: $$next/g" charts/podinfo/values.yaml && \
	perl -i -pe "s/tag: \Q$$current\E/tag: $$next/g" charts/podinfo/values-prod.yaml && \
	perl -i -pe "s/appVersion: \Q$$current\E/appVersion: $$next/g" charts/podinfo/Chart.yaml && \
	perl -i -pe "s/version: \Q$$current\E/version: $$next/g" charts/podinfo/Chart.yaml && \
	perl -i -pe "s/\Q$$current\E/$$next/g" timoni/podinfo/values.cue && \
	grep -rl "podinfo:$$current" deploy kustomize | xargs perl -i -pe "s/\Qpodinfo:$$current\E/podinfo:$$next/g" && \
	echo "Version $$next set in code, deployment, module, chart and kustomize"

prep-release:
	@branch="$$(git rev-parse --abbrev-ref HEAD)" && \
	if [ "$$branch" != "master" ]; then \
		echo "Error: prep-release must be run from the master branch (current: $$branch)"; \
		exit 1; \
	fi && \
	git pull origin master && \
	next="$(TAG)" && \
	if [ "$$next" = "latest" ]; then \
		next="$$(echo $(VERSION) | awk -F. '{ printf "%d.%d.%d", $$1, $$2+1, 0 }')"; \
	fi && \
	git checkout -b release-$$next && \
	$(MAKE) version-set TAG=$$next && \
	git commit -am "Release $$next" && \
	git push origin release-$$next && \
	gh pr create --title "Release $$next" --body "Prepare for $$next release" --base master --head release-$$next

release:
	git tag -s -m $(VERSION) $(VERSION)
	git push origin $(VERSION)

swagger:
	go install github.com/swaggo/swag/cmd/swag@latest
	go get github.com/swaggo/swag/gen@latest
	go get github.com/swaggo/swag/cmd/swag@latest
	cd pkg/api/http && $$(go env GOPATH)/bin/swag init -g server.go

.PHONY: timoni-build
timoni-build:
	@timoni build podinfo ./timoni/podinfo -f ./timoni/podinfo/debug_values.cue
