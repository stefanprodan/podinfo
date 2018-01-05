FROM alpine:latest

RUN apk add --no-cache curl openssl

ADD podinfo /podinfo

EXPOSE 9898
ENTRYPOINT ["/podinfo"]
