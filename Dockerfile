FROM alpine:latest

RUN apk add --no-cache curl openssl netcat-openbsd

ADD podinfo /podinfo

EXPOSE 9898
CMD ["./podinfo"]
