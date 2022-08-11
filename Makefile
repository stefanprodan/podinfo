# Makefile for releasing service
#
# The release version is controlled from pkg/version

TAG?=latest
NAME:=service
DOCKER_REPOSITORY:=feelguuds
DOCKER_IMAGE_NAME:=$(DOCKER_REPOSITORY)/$(NAME)
GIT_COMMIT:=$(shell git describe --dirty --always)
VERSION:=$(shell grep 'VERSION' pkg/version/version.go | awk '{ print $$4 }' | tr -d '"')
EXTRA_RUN_ARGS?=
DC=docker-compose -f ./compose/docker-compose-otel.yaml -f ./compose/docker-compose.yaml -f ./compose/docker-compose-dtm.yaml

.PHONY: help
.DEFAULT_GOAL := help

help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

stop: ## Stop all Docker Containers run in Compose
	$(DC) stop

clean: stop ## Clean all Docker Containers and Volumes
	$(DC) down --rmi local --remove-orphans -v
	$(DC) rm -f -v

build: clean ## Rebuild the Docker Image for use by Compose
	$(DC) build

start: stop ## Run the Application as a docker compose workflow
	$(DC) up

run-background: stop ## Run the Application
	$(DC) up --detach

run: ## Build and run the go binary
	go run -ldflags "-s -w -X github.com/stefanprodan/podinfo/pkg/version.REVISION=$(GIT_COMMIT)" cmd/podinfo/* \
	--level=debug --grpc-port=9999 --backend-url=https://httpbin.org/status/401 --backend-url=https://httpbin.org/status/500 \
	--ui-logo=https://raw.githubusercontent.com/stefanprodan/podinfo/gh-pages/cuddle_clap.gif $(EXTRA_RUN_ARGS)

build: ## Build the service binary
	GIT_COMMIT=$$(git rev-list -1 HEAD) && CGO_ENABLED=0 go build  -ldflags "-s -w -X github.com/SimifiniiCTO/simfiny-microservice-template/pkg/version.REVISION=$(GIT_COMMIT)" -a -o ./bin/service *
	GIT_COMMIT=$$(git rev-list -1 HEAD) && CGO_ENABLED=0 go build  -ldflags "-s -w -X github.com/SimifiniiCTO/simfiny-microservice-template/pkg/version.REVISION=$(GIT_COMMIT)" -a -o ./bin/podcli ./cmd/podcli/*

tidy: ## Tidy golang dependencies
	rm -f go.sum; go mod tidy -compat=1.17

fmt: ## Format codebase
	gofmt -l -s -w ./
	goimports -l -w ./

build-charts: ## Build and package helm charts
	helm lint charts/*
	helm package charts/*

build-container: ## Build docker container comprised of service code
	docker build -t $(DOCKER_IMAGE_NAME):$(VERSION) .

build-xx: ## Build docker container based off xx Dockerfile
	docker buildx build \
	--platform=linux/amd64 \
	-t $(DOCKER_IMAGE_NAME):$(VERSION) \
	--load \
	-f Dockerfile.xx .

build-base: ## Build docker container from base Dockerfile
	docker build -f Dockerfile.base -t $(DOCKER_REPOSITORY)/service-base:latest .

push-base: build-base ## Build and push docker image built from base Dockerfile
	docker push $(DOCKER_REPOSITORY)/service-base:latest

test-container: ## Test docker container
	@docker rm -f service || true
	@docker run -dp 9898:9898 --name=service $(DOCKER_IMAGE_NAME):$(VERSION)
	@docker ps
	@TOKEN=$$(curl -sd 'test' localhost:9898/token | jq -r .token) && \
	curl -sH "Authorization: Bearer $${TOKEN}" localhost:9898/token/validate | grep test

push-container: ## Push docker container
	docker tag $(DOCKER_IMAGE_NAME):$(VERSION) $(DOCKER_IMAGE_NAME):latest
	docker push $(DOCKER_IMAGE_NAME):$(VERSION)
	docker push $(DOCKER_IMAGE_NAME):latest
	docker tag $(DOCKER_IMAGE_NAME):$(VERSION) quay.io/$(DOCKER_IMAGE_NAME):$(VERSION)
	docker tag $(DOCKER_IMAGE_NAME):$(VERSION) quay.io/$(DOCKER_IMAGE_NAME):latest
	docker push quay.io/$(DOCKER_IMAGE_NAME):$(VERSION)
	docker push quay.io/$(DOCKER_IMAGE_NAME):latest

version-set: ## Set version
	@next="$(TAG)" && \
	current="$(VERSION)" && \
	/usr/bin/sed -i '' "s/$$current/$$next/g" pkg/version/version.go && \
	/usr/bin/sed -i '' "s/tag: $$current/tag: $$next/g" charts/service/values.yaml && \
	/usr/bin/sed -i '' "s/tag: $$current/tag: $$next/g" charts/service/values-prod.yaml && \
	/usr/bin/sed -i '' "s/appVersion: $$current/appVersion: $$next/g" charts/service/Chart.yaml && \
	/usr/bin/sed -i '' "s/version: $$current/version: $$next/g" charts/service/Chart.yaml && \
	/usr/bin/sed -i '' "s/service:$$current/service:$$next/g" kustomize/deployment.yaml && \
	/usr/bin/sed -i '' "s/service:$$current/service:$$next/g" deploy/webapp/frontend/deployment.yaml && \
	/usr/bin/sed -i '' "s/service:$$current/service:$$next/g" deploy/webapp/backend/deployment.yaml && \
	/usr/bin/sed -i '' "s/service:$$current/service:$$next/g" deploy/bases/frontend/deployment.yaml && \
	/usr/bin/sed -i '' "s/service:$$current/service:$$next/g" deploy/bases/backend/deployment.yaml && \
	/usr/bin/sed -i '' "s/$$current/$$next/g" cue/main.cue && \
	echo "Version $$next set in code, deployment, chart and kustomize"

release: ## Release service
	git tag $(VERSION)
	git push origin $(VERSION)

swagger: ## Generate swagger docs
	go install github.com/swaggo/swag/cmd/swag@latest
	go get github.com/swaggo/swag/gen@latest
	go get github.com/swaggo/swag/cmd/swag@latest
	cd pkg/api && $$(go env GOPATH)/bin/swag init -g server.go

.PHONY: cue-mod
cue-mod: ## Run cue mod
	@cd cue && cue get go k8s.io/api/...

.PHONY: cue-gen
cue-gen: ## Generate kubernetes artifacts based off of cue configs
	@cd cue && cue fmt ./... && cue vet --all-errors --concrete ./...
	@cd cue && cue gen

.PHONY: cover
cover: ## Run coverage report
	$(AT) rm -rf $(TMP_COVERAGE)
	$(AT) mkdir -p $(TMP_COVERAGE)
	go test $(GO_TEST_FLAGS) -json -cover -coverprofile=$(TMP_COVERAGE)/coverage.txt $(GO_PKGS) | tparse
	$(AT) go tool cover -html=$(TMP_COVERAGE)/coverage.txt -o $(TMP_COVERAGE)/coverage.html
	$(AT) echo
	$(AT) go tool cover -func=$(TMP_COVERAGE)/coverage.txt | grep total
	$(AT) echo
	$(AT) echo Open the coverage report:
	$(AT) echo open $(TMP_COVERAGE)/coverage.html
	$(AT) if [ "$(OPEN_COVERAGE_HTML)" == "1" ]; then open $(TMP_COVERAGE)/coverage.html; fi

.PHONY: go-mod
go-mod: ## List all dependencies
	go list -m -u all

.PHONY: unit-test
unit-test: run-background ## Run unit tests
	echo "starting unit tests and integration tests"
	go get github.com/mfridman/tparse
	go test -v -race ./... -json -cover  -coverprofile cover.out | tparse -all
	go tool cover -html=cover.out

.PHONY: unit-test
benchmark-test: run-background ## Run benchmark tests
	./benchmark/benchmark.sh

.PHONY: integration-test
integration-test: ## Run integration tests
	cd ./integration-tests/test.sh

gen-deps: ## Install dependencies for protoc auto gen
	export GOPATH=$(go env GOPATH)
	export PATH="$PATH:$(go env GOPATH)/bin"
	@echo "downloading protoc-gen tool"
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	go install github.com/pseudomuto/protoc-gen-doc/cmd/protoc-gen-doc
	go install github.com/infobloxopen/protoc-gen-gorm@latest
	go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2
	brew install protobuf protoc-gen-go protoc-gen-go-grpc

gen-service: ## Autogen service stubs
	@echo "setting up grpc service definition via protobuf"
	protoc -I. \
			-I=$(GOPATH)/src \
			-I=$(GOPATH)/src/github.com/infobloxopen/protoc-gen-gorm \
			-I=$(GOPATH)/src/github.com/infobloxopen/atlas-app-toolkit \
			-I=$(GOPATH)/src/github.com/lyft/protoc-gen-validate/validate/validate.proto \
			-I=$(GOPATH)/src/github.com/infobloxopen/protoc-gen-gorm/options \
			-I=$(GOPATH)/src/github.com/protobuf/src/google/protobuf/timestamp.proto \
			-I=$(GOPATH)/src/github.com/SimifiniiCTO/simfiny-microservice-template/api-definition/proto/schema/service_schema.proto \
			-I=$(GOPATH)/src/github.com/SimifiniiCTO/simfiny-microservice-template/api-definition/v1/service.proto \
			api-definition/v1/service.proto api-definition/proto/schema/service_schema.proto --go_out=:$(GOPATH)/src --go_opt=paths=import \
			--go-grpc_out=:$(GOPATH)/src --go-grpc_opt=paths=import

gen-data: ## Autogen service db schema
	@echo "setting up grpc schema definition via protobuf"
	protoc -I. \
			-I=$(GOPATH)/src \
			-I=$(GOPATH)/src/github.com/infobloxopen/protoc-gen-gorm \
			-I=$(GOPATH)/src/github.com/infobloxopen/atlas-app-toolkit \
			-I=$(GOPATH)/src/github.com/lyft/protoc-gen-validate/validate/validate.proto \
			-I=$(GOPATH)/src/github.com/infobloxopen/protoc-gen-gorm/options \
			-I=$(GOPATH)/src/github.com/protobuf/src/google/protobuf/timestamp.proto \
			-I=$(GOPATH)/src/github.com/SimifiniiCTO/simfiny-microservice-template/api-definition/proto/schema/service_schema.proto \
			api-definition/proto/schema/service_schema.proto --gorm_out="engine=postgres:$(GOPATH)/src"

gen-docs: ## Autogen service docs
	@echo "setting up grpc schema definition via protobuf"
	protoc -I. \
			-I=$(GOPATH)/src \
			-I=$(GOPATH)/src/github.com/infobloxopen/protoc-gen-gorm \
			-I=$(GOPATH)/src/github.com/infobloxopen/atlas-app-toolkit \
			-I=$(GOPATH)/src/github.com/lyft/protoc-gen-validate/validate/validate.proto \
			-I=$(GOPATH)/src/github.com/infobloxopen/protoc-gen-gorm/options \
			-I=$(GOPATH)/src/github.com/protobuf/src/google/protobuf/timestamp.proto \
			-I=$(GOPATH)/src/github.com/SimifiniiCTO/simfinii/src/backend/services/user-service/api/proto/schema/user_service_schema.proto \
			api-definition/proto/schema/service_schema.proto --doc_out=./documentation --doc_opt=markdown,schema.md

gen-rpc: ## Autogen service rpcs stubs
	buf generate

gen: gen-deps gen-service gen-data gen-rpc ## Run autogen suite
	@echo "generating grpc definitions"

.PHONY: skaffold
skaffold: ## Start skaffold
	minikube start --profile custom
	skaffold config set --global local-cluster true
	eval $(minikube -p custom docker-env)
	skaffold dev

.PHONY: kube-deploy-start
kube-deploy-start: ## Deploy service to minikube
	./scripts/local-build.sh
	./scripts/local-deploy.sh

.PHONY: kube-deploy-stop
kube-deploy-stop: ## Stop deployment of service to minikube
	./scripts/local-nuke.sh
