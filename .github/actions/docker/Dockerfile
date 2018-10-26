FROM docker:stable

LABEL "name"="Docker tag and push action"
LABEL "maintainer"="Stefan Prodan <support@gweave.works>"
LABEL "version"="1.0.0"

LABEL "com.github.actions.icon"="terminal"
LABEL "com.github.actions.color"="gray-dark"
LABEL "com.github.actions.name"="Docker Publish"
LABEL "com.github.actions.description"="This is an Action to run docker tag and push commands."

RUN apk update \
  && apk add --no-cache ca-certificates bash git curl \
  && (apk info | xargs -n1 -I{} apk --quiet add {}-doc); true \
  && rm -rf /var/cache/apk/*

COPY entrypoint.sh /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
