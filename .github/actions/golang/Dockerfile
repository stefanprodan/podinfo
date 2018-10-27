FROM golang:1.10

LABEL "name"="golang tools action"
LABEL "maintainer"="Stefan Prodan <support@weave.works>"
LABEL "version"="1.0.0"

LABEL "com.github.actions.icon"="code"
LABEL "com.github.actions.color"="green-dark"
LABEL "com.github.actions.name"="Go Tools"
LABEL "com.github.actions.description"="This is an Action to run go and dep commands."

ENV DEP_VERSION="v0.5.0"

RUN curl -fL -o /usr/local/bin/dep https://github.com/golang/dep/releases/download/${DEP_VERSION}/dep-linux-amd64 \
 && chmod +x /usr/local/bin/dep

COPY entrypoint.sh /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]

