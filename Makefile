# Makefile for releasing Alpine multi-arch Docker images
#
# The release version is controlled from pkg/version
#
# Prerequisites:
# 1) docker login (change the DOCKER_REPOSITORY to match your Docker Hub user)
# 2) go get github.com/estesp/manifest-tool

EMPTY:=
SPACE:=$(EMPTY) $(EMPTY)
COMMA:=$(EMPTY),$(EMPTY)
NAME:=podinfo
DOCKER_REPOSITORY:=stefanprodan
DOCKER_IMAGE_NAME:=$(DOCKER_REPOSITORY)/$(NAME)
GITREPO:=github.com/stefanprodan/k8s-podinfo
GITCOMMIT:=$(shell git describe --dirty --always)
VERSION:=$(shell grep 'VERSION' pkg/version/version.go | awk '{ print $$4 }' | tr -d '"')
LINUX_ARCH:=amd64 arm arm64 ppc64le s390x
PLATFORMS:=$(subst $(SPACE),$(COMMA),$(foreach arch,$(LINUX_ARCH),linux/$(arch)))

.PHONY: build
build:
	@echo Cleaning old builds
	@rm -rf build && mkdir build
	@echo Building: linux/$(LINUX_ARCH)  $(VERSION) ;\
	for arch in $(LINUX_ARCH); do \
	    mkdir -p build/linux/$$arch && CGO_ENABLED=0 GOOS=linux GOARCH=$$arch go build -ldflags="-s -w -X $(GITREPO)/pkg/version.GITCOMMIT=$(GITCOMMIT)" -o build/linux/$$arch/$(NAME) ./cmd/$(NAME) ;\
	done

.PHONY: tar
tar:
	@echo Cleaning old releases
	@rm -rf release && mkdir release
	for arch in $(LINUX_ARCH); do \
	    tar -zcf release/$(NAME)_$(VERSION)_linux_$$arch.tgz -C build/linux/$$arch $(NAME) ;\
	done

.PHONY: docker-build
docker-build: tar
	# Steps:
	# 1. Copy appropriate podinfo binary to build/docker/linux/<arch>
	# 2. Copy Dockerfile to build/docker/linux/<arch>
	# 3. Replace base image from alpine:latest to <arch>/alpine:latest
	# 4. Comment RUN in Dockerfile
	# <arch>:
	# arm: arm32v6
	# arm64: arm64v8
	rm -rf build/docker
	@for arch in $(LINUX_ARCH); do \
	    mkdir -p build/docker/linux/$$arch ;\
	    tar -xzf release/$(NAME)_$(VERSION)_linux_$$arch.tgz -C build/docker/linux/$$arch ;\
	    cp Dockerfile build/docker/linux/$$arch ;\
	    cp Dockerfile build/docker/linux/$$arch/Dockerfile.in ;\
	    if [ $$arch != amd64 ]; then \
		case $$arch in \
	        arm) \
	            BASEIMAGE=arm32v6 ;\
	            ;; \
	        arm64) \
	            BASEIMAGE=arm64v8 ;\
	            ;; \
	        *) \
	            BASEIMAGE=$$arch ;\
	            ;; \
	        esac ;\
	        sed -e "s/alpine:latest/$$BASEIMAGE\\/alpine:latest/" -e "s/^\\s*RUN/#RUN/" build/docker/linux/$$arch/Dockerfile.in > build/docker/linux/$$arch/Dockerfile ;\
	    fi ;\
	    docker build -t $(NAME) build/docker/linux/$$arch ;\
	    docker tag $(NAME) $(DOCKER_IMAGE_NAME):$(NAME)-$$arch ;\
	done

.PHONY: docker-push
docker-push:
	@echo Pushing: $(VERSION) to $(DOCKER_IMAGE_NAME)
	for arch in $(LINUX_ARCH); do \
	    docker push $(DOCKER_IMAGE_NAME):$(NAME)-$$arch ;\
	done
	manifest-tool push from-args --platforms $(PLATFORMS) --template $(DOCKER_IMAGE_NAME):podinfo-ARCH --target $(DOCKER_IMAGE_NAME):$(VERSION)
	manifest-tool push from-args --platforms $(PLATFORMS) --template $(DOCKER_IMAGE_NAME):podinfo-ARCH --target $(DOCKER_IMAGE_NAME):latest

.PHONY: quay-push
quay-push:
	@echo Pushing: $(VERSION) to quay.io/$(DOCKER_IMAGE_NAME):$(VERSION)
	@cd build/docker/linux/amd64/ ; docker build -t quay.io/$(DOCKER_IMAGE_NAME):$(VERSION) . ; docker push quay.io/$(DOCKER_IMAGE_NAME):$(VERSION)

.PHONY: clean
clean:
	rm -rf release
	rm -rf build

.PHONY: test
test:
	cd pkg/server ; go test -v -race ./...

.PHONY: dep
dep:
	go get -u github.com/golang/dep/cmd/dep
	go get -u github.com/estesp/manifest-tool

.PHONY: package
package:
	cd chart/stable/ && helm package podinfo/
	mv chart/stable/podinfo-0.1.0.tgz docs/
	cd chart/stable/ && helm package ambassador/
	mv chart/stable/ambassador-0.1.0.tgz docs/
	helm repo index docs --url https://stefanprodan.github.io/k8s-podinfo --merge ./docs/index.yaml
