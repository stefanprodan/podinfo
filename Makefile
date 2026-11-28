# Makefile for building, testing and releasing podinfo.

TAG?=latest
NAME:=podinfo
DOCKER_REPOSITORY:=stefanprodan
DOCKER_IMAGE_NAME:=$(DOCKER_REPOSITORY)/$(NAME)
GIT_COMMIT:=$(shell git describe --dirty --always)
VERSION:=$(shell grep 'VERSION' pkg/version/version.go | awk '{ print $$4 }' | tr -d '"')
EXTRA_RUN_ARGS?=

# Recipes run in bash and stop when a command or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

.PHONY: all
all: test build ## Run the tests and build the binaries.

##@ Development

.PHONY: run
run: ## Run podinfo locally with debug logging.
	go run -ldflags "-s -w -X github.com/stefanprodan/podinfo/pkg/version.REVISION=$(GIT_COMMIT)" ./cmd/podinfo \
	--level=debug --grpc-port=9999 $(EXTRA_RUN_ARGS)

.PHONY: run-ui-demo
run-ui-demo: ## Run the UI against a fake API that plays a canary release.
	@rm -rf bin/ui-demo && mkdir -p bin/ui-demo
	@perl -0pe 'BEGIN { local $$/; open F, "<", "ui/demo/demo.js" or die $$!; $$demo = <F> } s{<script src=}{<script type="module">\n$$demo</script>\n<script src=}' ui/index.html > bin/ui-demo/index.html
	go run ./cmd/podinfo --ui-path bin/ui-demo --port $${PORT:-9898} --level warn

.PHONY: tidy
tidy: ## Run go mod tidy.
	rm -f go.sum; go mod tidy

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: test
test: tidy fmt vet ## Run all unit tests.
	go test ./... -coverprofile cover.out

.PHONY: lint
lint: lint-ui lint-proto ## Run all linters.

.PHONY: lint-ui
lint-ui: ## Lint the web UI with Biome.
	biome ci .

.PHONY: lint-proto
lint-proto: ## Lint the protos and check their formatting with buf.
	buf lint
	buf format --exit-code --diff

##@ Generate

.PHONY: gen
gen: gen-proto gen-swagger gen-chart-schema vendor-swagger-ui ## Run all generators.

.PHONY: gen-proto
gen-proto: ## Generate the gRPC stubs with buf.
	buf generate

.PHONY: gen-swagger
gen-swagger: ## Generate the Swagger spec from the HTTP API annotations.
	go tool swag init -g server.go -d pkg/api/http -o pkg/api/http/docs --outputTypes json

SWAGGER_UI_VERSION:=5.36.0
SWAGGER_UI_SHA512:=2edebf29bd205e62dd89a6cb86fb28e157493fd57ba89581d33874833797515dfe08c393ccc9afa494525cdec40237f08b1c26a22f5ecc36cb1e27b10bb940f4

.PHONY: vendor-swagger-ui
vendor-swagger-ui: ## Vendor the Swagger UI from the swagger-ui-dist npm package.
	@tgz=$$(mktemp) && trap "rm -f $$tgz" EXIT && \
	curl -sSfL https://registry.npmjs.org/swagger-ui-dist/-/swagger-ui-dist-$(SWAGGER_UI_VERSION).tgz -o $$tgz && \
	echo "$(SWAGGER_UI_SHA512)  $$tgz" | shasum -a 512 -c - >/dev/null && \
	rm -rf ui/swagger/dist && mkdir ui/swagger/dist && \
	tar -xzf $$tgz -C ui/swagger/dist --strip-components=1 package/swagger-ui-bundle.js package/swagger-ui-bundle.js.LICENSE.txt package/swagger-ui.css package/LICENSE package/NOTICE && \
	echo "Swagger UI $(SWAGGER_UI_VERSION) vendored in ui/swagger/dist"

# values.schema.json is generated from values.yaml and its "@schema" comments.
# The root stays open for the keys Helm passes to a subchart, such as the dependency condition.
.PHONY: gen-chart-schema
gen-chart-schema: ## Generate the Helm chart values schema.
	cd charts/podinfo && go run github.com/losisin/helm-values-schema-json/v2@v2.4.0 -f values.yaml -o values.schema.json --draft 7 --indent 2 --no-additional-properties --schema-root.additional-properties

##@ Build

.PHONY: build
build: ## Build the podinfo and podcli binaries.
	CGO_ENABLED=0 go build -ldflags "-s -w -X github.com/stefanprodan/podinfo/pkg/version.REVISION=$(GIT_COMMIT)" -o ./bin/podinfo ./cmd/podinfo
	CGO_ENABLED=0 go build -ldflags "-s -w -X github.com/stefanprodan/podinfo/pkg/version.REVISION=$(GIT_COMMIT)" -o ./bin/podcli ./cmd/podcli

.PHONY: build-container
build-container: ## Build the container image.
	docker build -t $(DOCKER_IMAGE_NAME):$(VERSION) --build-arg REVISION=$(GIT_COMMIT) .

.PHONY: build-chart
build-chart: ## Lint and package the Helm chart.
	helm lint charts/*
	helm package charts/*

.PHONY: build-timoni
build-timoni: ## Build the Timoni module with the debug values.
	@timoni build podinfo ./timoni/podinfo -f ./timoni/podinfo/debug_values.cue

##@ Release

.PHONY: set-version
set-version: ## Set the TAG version in code, chart, module and manifests.
	@next="$(TAG)" && \
	current="$(VERSION)" && \
	perl -i -pe "s/\Q$$current\E/$$next/g" pkg/version/version.go && \
	perl -i -pe "s/tag: \Q$$current\E/tag: $$next/g" charts/podinfo/values.yaml && \
	perl -i -pe "s/appVersion: \Q$$current\E/appVersion: $$next/g" charts/podinfo/Chart.yaml && \
	perl -i -pe "s/version: \Q$$current\E/version: $$next/g" charts/podinfo/Chart.yaml && \
	perl -0777 -i -pe "s/(repository: \*\"ghcr.io\/stefanprodan\/podinfo\" \| string\n\s*tag:\s*\*\")\Q$$current\E\"/\$${1}$$next\"/" timoni/podinfo/images.cue && \
	grep -rl "podinfo:$$current" deploy kustomize | xargs perl -i -pe "s/\Qpodinfo:$$current\E/podinfo:$$next/g" && \
	echo "Version $$next set in code, deployment, module, chart and kustomize"

.PHONY: prep-release
prep-release: ## Create the release PR for TAG (defaults to the next minor).
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
	$(MAKE) set-version TAG=$$next && \
	git commit -am "Release $$next" && \
	git push origin release-$$next && \
	gh pr create --title "Release $$next" --body "Prepare for $$next release" --base master --head release-$$next

.PHONY: release
release: ## Tag the current version and push the tag.
	git tag -s -m $(VERSION) $(VERSION)
	git push origin $(VERSION)

##@ General

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)
