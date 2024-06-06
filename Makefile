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

.PHONY: increment-major increment-minor increment-patch

# Define the path to the version increment script
SCRIPT_PATH=./scripts/increment_version.sh

# Targets to increment major, minor, or patch versions
increment-major:
	@bash $(SCRIPT_PATH) major

increment-minor:
	@bash $(SCRIPT_PATH) minor

increment-patch:
	@bash $(SCRIPT_PATH) patch

run:
	go run -ldflags "-s -w -X github.com/stefanprodan/podinfo/pkg/version.REVISION=$(GIT_COMMIT)" cmd/podinfo/* \
	--level=debug --grpc-port=9999 --backend-url=https://httpbin.org/status/401 --backend-url=https://httpbin.org/status/500 \
	--ui-logo=https://raw.githubusercontent.com/stefanprodan/podinfo/gh-pages/cuddle_clap.gif $(EXTRA_RUN_ARGS)

.PHONY: test
test:
	go test ./... -coverprofile cover.out

build:
	GIT_COMMIT=$$(git rev-list -1 HEAD) && CGO_ENABLED=0 go build  -ldflags "-s -w -X github.com/stefanprodan/podinfo/pkg/version.REVISION=$(GIT_COMMIT)" -a -o ./bin/podinfo ./cmd/podinfo/*
	GIT_COMMIT=$$(git rev-list -1 HEAD) && CGO_ENABLED=0 go build  -ldflags "-s -w -X github.com/stefanprodan/podinfo/pkg/version.REVISION=$(GIT_COMMIT)" -a -o ./bin/podcli ./cmd/podcli/*

tidy:
	rm -f go.sum; go mod tidy -compat=1.22

vet:
	go vet ./...

fmt:
	gofmt -l -s -w ./
	goimports -l -w ./

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


# Target to commit the version change
git-commit-version:
	@echo "Committing version changes to Git..."
	git add .
	git commit -m "chore: bump version to $(VERSION)"
	git push origin main

# Function to update the version in pkg/version/version.go with the value of TAG
update-version-file:
	@echo "Updating version in version.go to $(TAG)..."
	@sed -i.bak -e "s/^var VERSION = \".*\"/var VERSION = \"$(TAG)\"/" pkg/version/version.go
	@rm -f pkg/version/version.go.bak
	@echo "Version updated to $(TAG) in version.go"

# Function to update the TAG in the Makefile
update-tag:
	$(eval NEW_VERSION := $(shell bash $(SCRIPT_PATH) $(version_type)))
	@echo "Updating TAG to $(NEW_VERSION)"
	@sed -i.bak -e "s/^TAG\?=.*$$/TAG\?=$(NEW_VERSION)/" Makefile
	@rm -f Makefile.bak
	@$(MAKE) update-version-file TAG=$(NEW_VERSION)

# Targets to increment major, minor, or patch versions and update the TAG
release-major: version_type=major
release-major: update-tag
	@echo "Released version $(TAG)"

release-minor: version_type=minor
release-minor: update-tag
	@echo "Released version $(TAG)"

release-patch: version_type=patch
release-patch: update-tag
	@echo "Released version $(TAG)"

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

# release-major: increment-major

# release-minor: increment-minor

# release-patch: increment-patch

swagger:
	go install github.com/swaggo/swag/cmd/swag@latest
	go get github.com/swaggo/swag/gen@latest
	go get github.com/swaggo/swag/cmd/swag@latest
	cd pkg/api/http && $$(go env GOPATH)/bin/swag init -g server.go

.PHONY: timoni-build
timoni-build:
	@timoni build podinfo ./timoni/podinfo -f ./timoni/podinfo/debug_values.cue

.PHONY: start
start:
	kubectx docker-desktop 
	devspace use namespace local
	devspace dev

.PHONY: stop
stop:
	devspace purge