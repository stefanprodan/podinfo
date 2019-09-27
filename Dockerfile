FROM golang:1.13 as builder

RUN mkdir -p /podinfo/

WORKDIR /podinfo

COPY . .

RUN go mod download

RUN go test -v -race ./...

RUN GIT_COMMIT=$(git rev-list -1 HEAD) && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags "-s -w \
    -X github.com/stefanprodan/podinfo/pkg/version.REVISION=${GIT_COMMIT}" \
    -a -o bin/podinfo cmd/podinfo/*

RUN GIT_COMMIT=$(git rev-list -1 HEAD) && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags "-s -w \
    -X github.com/stefanprodan/podinfo/pkg/version.REVISION=${GIT_COMMIT}" \
    -a -o bin/podcli cmd/podcli/*

FROM alpine:3.10

RUN addgroup -S app \
    && adduser -S -g app app \
    && apk --no-cache add \
    curl openssl netcat-openbsd

WORKDIR /home/app

COPY --from=builder /podinfo/bin/podinfo .
COPY --from=builder /podinfo/bin/podcli /usr/local/bin/podcli
COPY ./ui ./ui
RUN chown -R app:app ./

USER app

CMD ["./podinfo"]
