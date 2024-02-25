# Podinfo signed releases

Podinfo release assets such as the Helm chart and the Flux artifact
are published to GitHub Container Registry and are signed with
[Notation](https://github.com/notaryproject/notation).

## Generate signing keys

Generate a new signing key pair:

```sh
openssl genrsa -out podinfo.key 2048
openssl req -new -key podinfo.key -out podinfo.csr -config codesign.cnf
openssl x509 -req -days 1826 -in podinfo.csr -signkey podinfo.key -out notation.crt -extensions v3_req -extfile codesign.cnf
```
