FROM alpine:3.7

RUN addgroup -S app \
    && adduser -S -g app app \
    && apk --no-cache add \
    curl openssl netcat-openbsd

WORKDIR /home/app
COPY ./ui ./ui
ADD podinfo .

RUN chown -R app:app ./

USER app

CMD ["./podinfo"]
