FROM golang:1.11 as builder

ARG REPOSITORY
ARG SHA

RUN mkdir -p /go/src/github.com/${REPOSITORY}/

WORKDIR /go/src/github.com/${REPOSITORY}

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build \
  -ldflags "-s -w -X github.com/${REPOSITORY}/pkg/version.REVISION=${SHA}" \
  -a -installsuffix cgo -o podinfo ./cmd/podinfo \
  && mv podinfo /usr/local/bin/podinfo

RUN CGO_ENABLED=0 GOOS=linux go build \
  -ldflags "-s -w -X github.com/${REPOSITORY}/pkg/version.REVISION=${SHA}" \
  -a -installsuffix cgo -o podcli ./cmd/podcli \
  && mv podcli /usr/local/bin/podcli

FROM alpine:3.8

RUN addgroup -S app \
    && adduser -S -g app app \
    && apk --no-cache add \
    curl openssl netcat-openbsd

WORKDIR /home/app

COPY --from=builder /usr/local/bin/podinfo .
COPY --from=builder /usr/local/bin/podcli /usr/local/bin/podcli
COPY ./ui ./ui

RUN chown -R app:app ./

USER app

CMD ["./podinfo"]
