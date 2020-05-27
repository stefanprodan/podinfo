FROM stefanprodan/alpine-base:latest

COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

RUN apk --no-cache add git

ENTRYPOINT ["/entrypoint.sh"]
