# OpenFaaS + Istio 

### Install Istio

Download latest release:

```bash
curl -L https://git.io/getLatestIstio | sh -
```

Configure Istio with Prometheus, Jaeger and cert-manager:

```yaml
global:
  nodePort: false
  proxy:
    includeIPRanges: "10.28.0.0/14,10.7.240.0/20"

ingress:
  enabled: false

sidecarInjectorWebhook:
  enabled: true
  enableNamespacesByDefault: false

gateways:
  enabled: true

grafana:
  enabled: true

prometheus:
  enabled: true

servicegraph:
  enabled: true

tracing:
  enabled: true

certmanager:
  enabled: true
```

Save the above file as `istio-of.yaml` and install Istio with Helm:

```bash
helm upgrade --install istio ./install/kubernetes/helm/istio \
--namespace=istio-system \
-f ./istio-of.yaml
``` 

### Configure Istio Gateway with LE certs

Istio Gateway:

```yaml
apiVersion: networking.istio.io/v1alpha3
kind: Gateway
metadata:
  name: public-gateway
  namespace: istio-system
spec:
  selector:
    istio: ingressgateway
  servers:
  - port:
      number: 80
      name: http
      protocol: HTTP
    hosts:
    - "*"
    tls:
      httpsRedirect: true
  - port:
      number: 443
      name: https
      protocol: HTTPS
    hosts:
    - "*"
    tls:
      mode: SIMPLE
      privateKey: /etc/istio/ingressgateway-certs/tls.key
      serverCertificate: /etc/istio/ingressgateway-certs/tls.crt
```

Find the gateway public IP:

```bash
IP=$(kubectl -n istio-system describe svc/istio-ingressgateway | grep 'Ingress' | awk '{print $NF}')
```

Create a zone in GCP Cloud DNS with the following records:

```bash
istio.example.com. A $IP
*.istio.example.com. A $IP
```

Create a service account with Cloud DNS admin role (replace `my-gcp-project` with your project ID):

```bash
GCP_PROJECT=my-gcp-project

gcloud iam service-accounts create dns-admin \
--display-name=dns-admin \
--project=${GCP_PROJECT}

gcloud iam service-accounts keys create ./gcp-dns-admin.json \
--iam-account=dns-admin@${GCP_PROJECT}.iam.gserviceaccount.com \
--project=${GCP_PROJECT}

gcloud projects add-iam-policy-binding ${GCP_PROJECT} \
--member=serviceAccount:dns-admin@${GCP_PROJECT}.iam.gserviceaccount.com \
--role=roles/dns.admin
```

Create a Kubernetes secret with the GCP Cloud DNS admin key:

```bash
kubectl create secret generic cert-manager-credentials \
--from-file=./gcp-dns-admin.json \
--namespace=istio-system
```

LE issuer for GCP Cloud DNS:

```yaml
apiVersion: certmanager.k8s.io/v1alpha1
kind: Issuer
metadata:
  name: letsencrypt-prod
  namespace: istio-system
spec:
  acme:
    server: https://acme-v02.api.letsencrypt.org/directory
    email: email@example.com
    privateKeySecretRef:
      name: letsencrypt-prod
    dns01:
      providers:
      - name: cloud-dns
        clouddns:
          serviceAccountSecretRef:
            name: cert-manager-credentials
            key: gcp-dns-admin.json
          project: my-gcp-project
```

Wildcard cert:

```yaml
apiVersion: certmanager.k8s.io/v1alpha1
kind: Certificate
metadata:
  name: istio-gateway
  namespace: istio-system
spec:
  secretname: istio-ingressgateway-certs
  issuerRef:
    name: letsencrypt-prod
  commonName: "*.istio.example.com"
  dnsNames:
  - istio.example.com
  acme:
    config:
    - dns01:
        provider: cloud-dns
      domains:
      - "*.istio.example.com"
      - "istio.example.com"
```

### Configure OpenFaaS

Create the OpenFaaS namespaces:

```yaml
apiVersion: v1
kind: Namespace
metadata:
  labels:
    istio-injection: enabled
  name: openfaas
---
apiVersion: v1
kind: Namespace
metadata:
  labels:
    istio-injection: enabled
  name: openfaas-fn
```

Create an Istio virtual service for OpenFaaS Gateway:

```yaml
apiVersion: networking.istio.io/v1alpha3
kind: VirtualService
metadata:
  name: gateway
  namespace: openfaas
spec:
  hosts:
  - "openfaas.istio.example.com"
  gateways:
  - public-gateway.istio-system.svc.cluster.local
  http:
  - route:
    - destination:
        host: gateway
    timeout: 30s
```

Enable mTLS on `openfaas` namespace:

```yaml
apiVersion: authentication.istio.io/v1alpha1
kind: Policy
metadata:
  name: default
  namespace: openfaas
spec:
  peers:
  - mtls: {}
---
apiVersion: networking.istio.io/v1alpha3
kind: DestinationRule
metadata:
  name: default
  namespace: openfaas
spec:
  host: "*.openfaas.svc.cluster.local"
  trafficPolicy:
    tls:
      mode: ISTIO_MUTUAL
```

Allow plaintext traffic to OpenFaaS Gateway:

```yaml
apiVersion: authentication.istio.io/v1alpha1
kind: Policy
metadata:
  name: permissive
  namespace: openfaas
spec:
  targets:
  - name: gateway
  peers:
  - mtls:
      mode: PERMISSIVE
```

Enable mTLS on `openfaas-fn` namespace:

```yaml
apiVersion: authentication.istio.io/v1alpha1
kind: Policy
metadata:
  name: default
  namespace: openfaas-fn
spec:
  peers:
  - mtls: {}
---
apiVersion: networking.istio.io/v1alpha3
kind: DestinationRule
metadata:
  name: default
  namespace: openfaas-fn
spec:
  host: "*.openfaas-fn.svc.cluster.local"
  trafficPolicy:
    tls:
      mode: ISTIO_MUTUAL
```

Deny access to OpenFaaS core services from the `openfaas-fn` namespace except for system functions:

```yaml
apiVersion: config.istio.io/v1alpha2
kind: denier
metadata:
  name: denyhandler
  namespace: openfaas
spec:
  status:
    code: 7
    message: Not allowed
---
apiVersion: config.istio.io/v1alpha2
kind: checknothing
metadata:
  name: denyrequest
  namespace: openfaas
spec:
---
apiVersion: config.istio.io/v1alpha2
kind: rule
metadata:
  name: denyopenfaasfn
  namespace: openfaas
spec:
  match: destination.namespace == "openfaas" && source.namespace == "openfaas-fn" && source.labels["role"] != "openfaas-system"
  actions:
  - handler: denyhandler.denier
    instances: [ denyrequest.checknothing ]
```

Deny access to functions except for OpenFaaS core services:

```yaml
apiVersion: config.istio.io/v1alpha2
kind: denier
metadata:
  name: denyhandler
  namespace: openfaas-fn
spec:
  status:
    code: 7
    message: Not allowed
---
apiVersion: config.istio.io/v1alpha2
kind: checknothing
metadata:
  name: denyrequest
  namespace: openfaas-fn
spec:
---
apiVersion: config.istio.io/v1alpha2
kind: rule
metadata:
  name: denyopenfaasfn
  namespace: openfaas-fn
spec:
  match: destination.namespace == "openfaas-fn" && source.namespace != "openfaas" && source.labels["role"] != "openfaas-system"
  actions:
  - handler: denyhandler.denier
    instances: [ denyrequest.checknothing ]
```

### Install OpenFaaS

Add the OpenFaaS `helm` chart:

```bash
$ helm repo add openfaas https://openfaas.github.io/faas-netes/
```

Create a secret named `basic-auth` in the `openfaas` namespace:

```bash
# generate a random password
password=$(head -c 12 /dev/urandom | shasum| cut -d' ' -f1)

kubectl -n openfaas create secret generic basic-auth \
--from-literal=basic-auth-user=admin \
--from-literal=basic-auth-password=$password 
```

Install OpenFaaS with Helm:

```bash
helm upgrade --install openfaas ./chart/openfaas \
--namespace openfaas \
--set functionNamespace=openfaas-fn \
--set operator.create=true \
--set securityContext=true \
--set basic_auth=true \
--set exposeServices=false \
--set operator.createCRD=true \
--set gateway.image=stefanprodan/gateway:istio5
```

Wait for OpenFaaS Gateway to come online:

```bash
watch curl -v http://openfaas.istio.example.com/heathz 
```

Save your credentials in faas-cli store:

```bash
echo $password | faas-cli login -g https://openfaas.istio.example.com -u admin --password-stdin
```

