# Makefile for releasing Alpine multi-arch Docker images
#
# The release version is controlled from pkg/version
#
# Prerequsitesx:


EMPTY:=
SPACE:=$(EMPTY) $(EMPTY)
COMMA:=$(EMPTY),$(EMPTY)

ifeq (, $(shell which manifest-tool))
    $(error "No manifest-tool in $$PATH, install with: go get github.com/estesp/manifest-tool")
endif

DOCKER_REPOSITORY:=stefanprodan
NAME:=podinfo
VERSION:=$(shell grep 'VERSION' pkg/version/version.go | awk '{ print $$4 }' | tr -d '"')
DOCKER_IMAGE_NAME:=$(DOCKER_REPOSITORY)/$(NAME)
GITCOMMIT:=$(shell git describe --dirty --always)
LINUX_ARCH:=amd64 arm arm64 ppc64le
PLATFORMS:=$(subst $(SPACE),$(COMMA),$(foreach arch,$(LINUX_ARCH),linux/$(arch)))

all:
	@echo Use the 'release' target to start a release

release: build tar

docker: docker-build docker-push

.PHONY: build
build:
	@echo Cleaning old builds
	@rm -rf build && mkdir build
	@echo Building: darwin $(VERSION)
	mkdir -p build/darwin/amd64 && CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w -X github.com/stefanprodan/k8s-podinfo/pkg/version.GITCOMMIT=$(GITCOMMIT)" -o build/darwin/amd64/$(NAME) ./cmd/podinfo
	@echo Building: linux/$(LINUX_ARCH)  $(VERSION) ;\
	for arch in $(LINUX_ARCH); do \
	    mkdir -p build/linux/$$arch && CGO_ENABLED=0 GOOS=linux GOARCH=$$arch go build -ldflags="-s -w -X github.com/stefanprodan/k8s-podinfo/pkg/version.GITCOMMIT=$(GITCOMMIT)" -o build/linux/$$arch/$(NAME) ./cmd/podinfo ;\
	done

.PHONY: tar
tar:
	@echo Cleaning old releases
	@rm -rf release && mkdir release
	tar -zcf release/$(NAME)_$(VERSION)_darwin_amd64.tgz -C build/darwin/amd64 $(NAME)
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
	        sed -e "s/alpine:latest/$$BASEIMAGE\\/alpine:latest/" build/docker/linux/$$arch/Dockerfile.in > build/docker/linux/$$arch/Dockerfile ;\
	    fi ;\
	    docker build -t podinfo build/docker/linux/$$arch ;\
	    docker tag podinfo $(DOCKER_IMAGE_NAME):podinfo-$$arch ;\
	done

.PHONY: docker-push
docker-push:
	@echo Pushing: $(VERSION) to $(DOCKER_IMAGE_NAME)
	for arch in $(LINUX_ARCH); do \
	    docker push $(DOCKER_IMAGE_NAME):podinfo-$$arch ;\
	done
	manifest-tool push from-args --platforms $(PLATFORMS) --template $(DOCKER_IMAGE_NAME):podinfo-ARCH --target $(DOCKER_IMAGE_NAME):$(VERSION)
	manifest-tool push from-args --platforms $(PLATFORMS) --template $(DOCKER_IMAGE_NAME):podinfo-ARCH --target $(DOCKER_IMAGE_NAME):latest

.PHONY: clean
clean:
	rm -rf release
	rm -rf build
