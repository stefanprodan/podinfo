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
	rm -f go.sum; go mod tidy -compat=1.25

vet:
	go vet ./...

fmt:
	go fmt ./...

build-charts:
	helm lint charts/*
	helm package charts/*

build-container:
	docker build -t $(DOCKER_IMAGE_NAME):$(VERSION) .

build-xx:
	docker buildx build \
	--platform=linux/amd64 \
	-t $(DOCKER_IMAGE_NAME):$(VERSION) \
	--load \
	-f Dockerfile.xx .

build-base:
	docker build -f Dockerfile.base -t $(DOCKER_REPOSITORY)/podinfo-base:latest .

push-base: build-base
	docker push $(DOCKER_REPOSITORY)/podinfo-base:latest

test-container:
	@docker rm -f podinfo || true
	@docker run -dp 9898:9898 --name=podinfo $(DOCKER_IMAGE_NAME):$(VERSION)
	@docker ps
	@TOKEN=$$(curl -sd 'test' localhost:9898/token | jq -r .token) && \
	curl -sH "Authorization: Bearer $${TOKEN}" localhost:9898/token/validate | grep test

push-container:
	docker tag $(DOCKER_IMAGE_NAME):$(VERSION) $(DOCKER_IMAGE_NAME):latest
	docker push $(DOCKER_IMAGE_NAME):$(VERSION)
	docker push $(DOCKER_IMAGE_NAME):latest
	docker tag $(DOCKER_IMAGE_NAME):$(VERSION) quay.io/$(DOCKER_IMAGE_NAME):$(VERSION)
	docker tag $(DOCKER_IMAGE_NAME):$(VERSION) quay.io/$(DOCKER_IMAGE_NAME):latest
	docker push quay.io/$(DOCKER_IMAGE_NAME):$(VERSION)
	docker push quay.io/$(DOCKER_IMAGE_NAME):latest

version-set:
	@next="$(TAG)" && \
	current="$(VERSION)" && \
	/usr/bin/sed -i '' "s/$$current/$$next/g" pkg/version/version.go && \
	/usr/bin/sed -i '' "s/tag: $$current/tag: $$next/g" charts/podinfo/values.yaml && \
	/usr/bin/sed -i '' "s/tag: $$current/tag: $$next/g" charts/podinfo/values-prod.yaml && \
	/usr/bin/sed -i '' "s/appVersion: $$current/appVersion: $$next/g" charts/podinfo/Chart.yaml && \
	/usr/bin/sed -i '' "s/version: $$current/version: $$next/g" charts/podinfo/Chart.yaml && \
	/usr/bin/sed -i '' "s/podinfo:$$current/podinfo:$$next/g" kustomize/deployment.yaml && \
	/usr/bin/sed -i '' "s/podinfo:$$current/podinfo:$$next/g" deploy/webapp/frontend/deployment.yaml && \
	/usr/bin/sed -i '' "s/podinfo:$$current/podinfo:$$next/g" deploy/webapp/backend/deployment.yaml && \
	/usr/bin/sed -i '' "s/podinfo:$$current/podinfo:$$next/g" deploy/bases/frontend/deployment.yaml && \
	/usr/bin/sed -i '' "s/podinfo:$$current/podinfo:$$next/g" deploy/bases/backend/deployment.yaml && \
	/usr/bin/sed -i '' "s/$$current/$$next/g" timoni/podinfo/values.cue && \
	echo "Version $$next set in code, deployment, module, chart and kustomize"

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
