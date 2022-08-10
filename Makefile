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

run:
	go run -ldflags "-s -w -X github.com/stefanprodan/podinfo/pkg/version.REVISION=$(GIT_COMMIT)" cmd/podinfo/* \
	--level=debug --grpc-port=9999 --backend-url=https://httpbin.org/status/401 --backend-url=https://httpbin.org/status/500 \
	--ui-logo=https://raw.githubusercontent.com/stefanprodan/podinfo/gh-pages/cuddle_clap.gif $(EXTRA_RUN_ARGS)

.PHONY: test
test:
	go test ./... -coverprofile cover.out

build:
	GIT_COMMIT=$$(git rev-list -1 HEAD) && CGO_ENABLED=0 go build  -ldflags "-s -w -X github.com/SimifiniiCTO/simfiny-microservice-template/pkg/version.REVISION=$(GIT_COMMIT)" -a -o ./bin/service *
	GIT_COMMIT=$$(git rev-list -1 HEAD) && CGO_ENABLED=0 go build  -ldflags "-s -w -X github.com/SimifiniiCTO/simfiny-microservice-template/pkg/version.REVISION=$(GIT_COMMIT)" -a -o ./bin/podcli ./cmd/podcli/*

tidy:
	rm -f go.sum; go mod tidy -compat=1.17

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
	docker build -f Dockerfile.base -t $(DOCKER_REPOSITORY)/service-base:latest .

push-base: build-base
	docker push $(DOCKER_REPOSITORY)/service-base:latest

test-container:
	@docker rm -f service || true
	@docker run -dp 9898:9898 --name=service $(DOCKER_IMAGE_NAME):$(VERSION)
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

release:
	git tag $(VERSION)
	git push origin $(VERSION)

swagger:
	go install github.com/swaggo/swag/cmd/swag@latest
	go get github.com/swaggo/swag/gen@latest
	go get github.com/swaggo/swag/cmd/swag@latest
	cd pkg/api-definition && $$(go env GOPATH)/bin/swag init -g server.go

.PHONY: cue-mod
cue-mod:
	@cd cue && cue get go k8s.io/api-definition/...

.PHONY: cue-gen
cue-gen:
	@cd cue && cue fmt ./... && cue vet --all-errors --concrete ./...
	@cd cue && cue gen

# start docker containers in the backgound
.PHONY: up-d
up-d:
	echo "starting user service"
	docker-compose -f ./compose/docker-compose.yaml -f ./compose/docker-compose-dtm.yaml up --remove-orphans --detach

# stop all docker containers
.PHONY: down
down: 
	docker-compose -f ./compose/docker-compose.yaml -f ./compose/docker-compose-dtm.yaml down --remove-orphans

# start docker containers with logs running in the foreground
.PHONY: up
up:
	docker-compose -f ./compose/docker-compose.yaml -f ./compose/docker-compose-dtm.yaml up --remove-orphans

##
# Cover runs go_test on GO_PKGS and produces code coverage in multiple formats.
# A coverage.html file for human viewing will be at $(TMP_COVERAGE)/coverage.html
# This target will echo "open $(TMP_COVERAGE)/coverage.html" with TMP_COVERAGE
# expanded  so that you can easily copy "open $(TMP_COVERAGE)/coverage.html" into
# your terminal as a command to run, and then see the code coverage output locally.
.PHONY: cover
cover:
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
go-mod:
	go list -m -u all

.PHONY: ci-test
ci-test: compose-up-d
	echo "waiting for services to be ready to accept connections"
	sleep 30
	go test -v -race ./...

.PHONY: unit-test
unit-test: up-d
	echo "starting unit tests and integration tests"
	docker ps -a
	go get github.com/mfridman/tparse
	go test -v -race ./... -json -cover  -coverprofile cover.out | tparse -all
	go tool cover -html=cover.out

.PHONY: unit-test
benchmark-test: up-d
	./benchmark/benchmark.sh

.PHONY: integration-test
integration-test:
	cd ./integration-tests/test.sh

get-deps:
	export GOPATH=$(go env GOPATH)
	export PATH="$PATH:$(go env GOPATH)/bin"
	@echo "downloading protoc-gen tool"
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	go install github.com/pseudomuto/protoc-gen-doc/cmd/protoc-gen-doc
	go install github.com/infobloxopen/protoc-gen-gorm@latest
	go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2
	brew install protobuf protoc-gen-go protoc-gen-go-grpc

gen-service:
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

gen-data:
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

gen-rpc:
	buf generate

gen: get-deps gen-service  gen-data gen-rpc #gen-data
	@echo "generating grpc definitions"

.PHONY: start-skaffold
start-skaffold:
	minikube start --profile custom
	skaffold config set --global local-cluster true
	eval $(minikube -p custom docker-env)
	skaffold dev

.PHONY: start-minikube-deployment
start-minikube-deployment:
	./scripts/local-build.sh
	./scripts/local-deploy.sh

.PHONY: stop-minikube-deployment
stop-minikube-deployment:
	./scripts/local-nuke.sh
