FROM golang:1.11 as builder

RUN mkdir -p /go/src/github.com/stefanprodan/k8s-podinfo/

WORKDIR /go/src/github.com/stefanprodan/k8s-podinfo

COPY . .

RUN GIT_COMMIT=$(git rev-list -1 HEAD) && CGO_ENABLED=0 GOOS=linux \
    go build -ldflags "-s -w -X github.com/stefanprodan/k8s-podinfo/pkg/version.REVISION=${GIT_COMMIT}" \
    -a -installsuffix cgo -o podinfo ./cmd/podinfo

RUN GIT_COMMIT=$(git rev-list -1 HEAD) && CGO_ENABLED=0 GOOS=linux \
    go build -ldflags "-s -w -X github.com/stefanprodan/k8s-podinfo/pkg/version.REVISION=${GIT_COMMIT}" \
    -a -installsuffix cgo -o podcli ./cmd/podcli

FROM alpine:3.8

RUN addgroup -S app \
    && adduser -S -g app app \
    && apk --no-cache add \
    curl openssl netcat-openbsd

WORKDIR /home/app

COPY --from=builder /go/src/github.com/stefanprodan/k8s-podinfo/podinfo .
COPY --from=builder /go/src/github.com/stefanprodan/k8s-podinfo/podcli /usr/local/bin/podcli
COPY ./ui ./ui
RUN chown -R app:app ./

USER app

CMD ["./podinfo"]
