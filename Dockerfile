FROM golang:1.18-alpine as builder

ARG REVISION

RUN apk update
RUN apk add git

RUN mkdir -p /go/src/github.com/SimifiniiCTO/simfiny-microservice-template/

WORKDIR /go/src/github.com/SimifiniiCTO/simfiny-microservice-template

COPY . .

RUN git config --global url."https://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:x-oauth-basic@github.com/SimifiniiCTO".insteadOf "https://github.com/SimifiniiCTO"

RUN go mod download
RUN CGO_ENABLED=0 go build -ldflags "-s -w -X github.com/SimifiniiCTO/simfiny-microservice-template/pkg/version.REVISION=${REVISION}" -a -o main main.go

RUN set -ex && apk --no-cache add sudo curl
RUN apk update && apk add bash

RUN curl -Ls https://download.newrelic.com/install/newrelic-cli/scripts/install.sh | bash && sudo NEW_RELIC_API_KEY=NRAK-FGBHISKJUMWR9DXMIV4Q4EFZJTK NEW_RELIC_ACCOUNT_ID=3270596 /usr/local/bin/newrelic install -n logs-integration

CMD ["./main"]
