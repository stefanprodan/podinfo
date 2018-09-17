# OpenFaaS + Istio 

This is a guide on how to set up OpenFaaS on Google Kubernetes Engine (GKE) with Istio service mesh.

At the end of this guide you will be running OpenFaaS with the following characteristics:

* secure OpenFaaS ingress with Let’s Encrypt TLS and authentication
* encrypted communication between OpenFaaS core services and functions with Istio mutual TLS
* isolated functions with Istio Mixer rules
* Jaeger tracing and Prometheus monitoring for function calls
* canary deployments for OpenFaaS functions 

![openfaas-istio](https://github.com/stefanprodan/k8s-podinfo/blob/master/docs/screens/openfaas-istio-diagram.png)

### GKE cluster setup

You will be creating a cluster on Google’s Kubernetes Engine (GKE), 
if you don’t have an account you can sign up [here](https://cloud.google.com/free/) for free credit.

Login into GCP, create a project and enable billing for it. 
You should also enable the GKE service and Cloud DNS, 
from the left-hand menu navigate to `Compute -> Kubernetes Engine` and `Networking -> Network Services -> Cloud DNS`.

Install the [gcloud](https://cloud.google.com/sdk/) command line utility and configure your project with `gcloud init`.

Set default compute region and zone:

```bash
gcloud config set compute/region europe-west3
gcloud config set compute/zone europe-west3-a
```

Create a cluster with three nodes using the latest Kubernetes version:

```bash
k8s_version=$(gcloud container get-server-config --format=json \
| jq -r '.validNodeVersions[0]')

gcloud container clusters create openfaas \
--cluster-version=${k8s_version} \
--zone=europe-west3-a \
--num-nodes=3 \
--machine-type=n1-highcpu-4 \
--preemptible \
--no-enable-cloud-logging \
--disk-size=30 \
--enable-autorepair \
--scopes=gke-default,compute-rw,storage-rw
```

The above command will create a default node pool consisting of `n1-highcpu-4` (vCPU: 4, RAM 3.60GB, DISK: 30GB) preemptible VMs.
Preemptible VMs are up to 80% cheaper than regular instances and are terminated and replaced after a maximum of 24 hours.

Set up credentials for `kubectl`:

```bash
gcloud container clusters get-credentials openfaas -z=europe-west3-a
```

Create a cluster admin role binding:

```bash
kubectl create clusterrolebinding "cluster-admin-$(whoami)" \
--clusterrole=cluster-admin \
--user="$(gcloud config get-value core/account)"
```

Validate your setup with:

```bash
kubectl get nodes -o wide
```

### Cloud DNS setup

You will need an internet domain and access to the registrar to change the name servers to GCP Cloud DNS.

Create a managed zone named `openfaas` in Cloud DNS (replace `example.com` with your domain):

```bash
gcloud dns managed-zones create \
--dns-name="example.com." \
--description="OpenFaaS zone" "openfaas"
```

Look up your zone's name servers:

```bash
gcloud dns managed-zones describe openfaas
```

Update your registrar's name server records with the records returned by the above command. 

Wait for the name servers to change (replace `example.com` with your domain):

```bash
wait dig +short NS example.com
```

Create a static IP address named `istio-gateway-ip` in the same region as your GKE cluster:

```bash
gcloud compute addresses create istio-gateway-ip --region europe-west3-a
```

Find the static IP address:

```bash
gcloud compute addresses describe istio-gateway-ip --region europe-west3-a
```

Create the following DNS records (replace `example.com` with your domain and set your Istio Gateway IP):

```bash
DOMAIN="example.com"
GATEWAYIP="35.198.98.90"

gcloud dns record-sets transaction start --zone=openfaas

gcloud dns record-sets transaction add --zone=openfaas \
--name="${DOMAIN}" --ttl=300 --type=A ${GATEWAYIP}

gcloud dns record-sets transaction add --zone=openfaas \
--name="www.${DOMAIN}" --ttl=300 --type=CNAME ${DOMAIN}

gcloud dns record-sets transaction add --zone=openfaas \
--name="*.${DOMAIN}" --ttl=300 --type=A ${GATEWAYIP}

gcloud dns record-sets transaction execute --zone openfaas
```

Verify that the wildcard DNS is working (replace `example.com` with your domain):

```bash
watch host test.example.com
```

Find the GKE IP ranges:

```bash
gcloud container clusters describe openfaas --zone=europe-west3-a \
| grep -e clusterIpv4Cidr -e servicesIpv4Cidr
```

You'll be using the IP ranges to allow unrestricted egress traffic for services running inside the service mesh. 

### Install Istio

You will be using Helm to install Istio. Install Helm CLI with Homebrew:

```bash
brew install kubernetes-helm
```

Download the latest Istio release:
                                         
```bash
curl -L https://git.io/getLatestIstio | sh -
```

Create a service account and a cluster role binding for Tiller:

```bash
kubectl apply -f ./install/kubernetes/helm/helm-service-account.yaml
```

Deploy Tiller in the `kube-system` namespace:

```bash
helm init --service-account tiller
```

Configure Istio with Prometheus, Jaeger and cert-manager:

```yaml
global:
  nodePort: false
  proxy:
    # replace with your GKE IP ranges
    includeIPRanges: "10.28.0.0/14,10.7.240.0/20"

sidecarInjectorWebhook:
  enabled: true
  enableNamespacesByDefault: false

gateways:
  enabled: true
  istio-ingressgateway:
    replicaCount: 2
    autoscaleMin: 2
    autoscaleMax: 3
    # replace with your Istio Gateway IP
    loadBalancerIP: "35.198.98.90"
    type: LoadBalancer

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

Save the above file as `of-istio.yaml` and install Istio with Helm:

```bash
helm upgrade --install istio ./install/kubernetes/helm/istio \
--namespace=istio-system \
-f ./of-istio.yaml
``` 

### Configure Istio Gateway with Let's Encrypt wildcard certificate

![istio-letsencrypt](https://github.com/stefanprodan/k8s-podinfo/blob/master/docs/diagrams/istio-cert-manager-gcp.png)

Create a Istio Gateway in istio-system namespace with HTTPS redirect:

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

Save the above resource as istio-gateway.yaml and then apply it:

```bash
kubectl apply -f ./istio-gateway.yaml
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

Create a letsencrypt issuer for CloudDNS (replace `email@example.com` with a valid email address and `my-gcp-project` with your project ID):

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

Save the above resource as letsencrypt-issuer.yaml and then apply it:

```bash
kubectl apply -f ./letsencrypt-issuer.yaml
```

Create a wildcard certificate (replace `example.com` with your domain):

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
  commonName: "*.example.com"
  dnsNames:
  - istio.example.com
  acme:
    config:
    - dns01:
        provider: cloud-dns
      domains:
      - "*.example.com"
      - "example.com"
```

Save the above resource as of-cert.yaml and then apply it:

```bash
kubectl apply -f ./of-cert.yaml
```

In a couple of seconds cert-manager should fetch a wildcard certificate from letsencrypt.org:

```bash
kubectl -n istio-system logs deployment/certmanager -f
Certificate issued successfully
```

Recreate Istio ingress gateway pods:

```bash
kubectl -n istio-system delete pods -l istio=ingressgateway
```

Note that Istio gateway doesn't reload the certificates from the TLS secret on cert-manager renewal. 
Since the GKE cluster is made out of preemptible VMs the gateway pods will be replaced once every 24h, if your not using 
preemptible nodes then you need to manually kill the gateway pods every two months before the certificate expires.

### Configure OpenFaaS Gateway to receive external traffic

Create the OpenFaaS namespaces with Istio sidecar injection enabled:

```bash
kubectl apply -f https://raw.githubusercontent.com/openfaas/faas-netes/master/namespaces.yml
```

Create an Istio virtual service for OpenFaaS Gateway (replace `example.com` with your domain):

```yaml
apiVersion: networking.istio.io/v1alpha3
kind: VirtualService
metadata:
  name: gateway
  namespace: openfaas
spec:
  hosts:
  - "openfaas.example.com"
  gateways:
  - public-gateway.istio-system.svc.cluster.local
  http:
  - route:
    - destination:
        host: gateway
    timeout: 30s
```

Save the above resource as of-virtual-service.yaml and then apply it:

```bash
kubectl apply -f ./of-virtual-service.yaml
```

### Configure OpenFaaS mTLS and access policies

An OpenFaaS instance is composed out of two namespaces: one for the core services and one for functions. 
Kubernetes namespaces alone offer only a logical separation between workloads.
In order to secure the communication between core services and functions we need to enable mutual TLS on both namespaces.
To prohibit functions from calling each other or from reaching the OpenFaaS core services we need to create Istio Mixer rules.

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

Save the above resource as of-mtls.yaml and then apply it:

```bash
kubectl apply -f ./of-mtls.yaml
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

Save the above resource as of-gateway-mtls.yaml and then apply it:

```bash
kubectl apply -f ./of-gateway-mtls.yaml
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

Save the above resource as of-functions-mtls.yaml and then apply it:

```bash
kubectl apply -f ./of-functions-mtls.yaml
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

Save the above resources as of-rules.yaml and then apply it:

```bash
kubectl apply -f ./of-rules.yaml
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

Save the above resources as of-functions-rules.yaml and then apply it:

```bash
kubectl apply -f ./of-functions-rules.yaml
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
--set operator.createCRD=true
```

Wait for OpenFaaS Gateway to come online:

```bash
watch curl -v https://openfaas.example.com/heathz 
```

Save your credentials in faas-cli store:

```bash
echo $password | faas-cli login -g https://openfaas.example.com -u admin --password-stdin
```

### Canary deployments for OpenFaaS functions

![openfaas-canary](https://github.com/stefanprodan/k8s-podinfo/blob/master/docs/screens/openfaas-istio-canary.png)


Create a general available release for the `env` function version 1.0.0:

```yaml
apiVersion: openfaas.com/v1alpha2
kind: Function
metadata:
  name: env
  namespace: openfaas-fn
spec:
  name: env
  image: stefanprodan/of-env:1.0.0
  resources:
    requests:
      memory: "32Mi"
      cpu: "10m"
  limits:
    memory: "64Mi"
    cpu: "100m"
```

Save the above resources as env-ga.yaml and then apply it:

```bash
kubectl apply -f ./env-ga.yaml
```

Create a canary release for version 1.1.0:

```yaml
apiVersion: openfaas.com/v1alpha2
kind: Function
metadata:
  name: env-canary
  namespace: openfaas-fn
spec:
  name: env-canary
  image: stefanprodan/of-env:1.1.0
  resources:
    requests:
      memory: "32Mi"
      cpu: "10m"
  limits:
    memory: "64Mi"
    cpu: "100m"
```

Save the above resources as env-canary.yaml and then apply it:

```bash
kubectl apply -f ./env-canaray.yaml
```

Create an Istio virtual service with 10% traffic going to canary:

```yaml
apiVersion: networking.istio.io/v1alpha3
kind: VirtualService
metadata:
  name: env
  namespace: openfaas-fn
spec:
  hosts:
  - env
  http:
  - route:
    - destination:
        host: env
      weight: 90
    - destination:
        host: env-canary
      weight: 10
    timeout: 30s
```

Save the above resources as env-virtual-service.yaml and then apply it:

```bash
kubectl apply -f ./env-virtual-service.yaml
```

Test traffic routing (one in ten calls should hit the canary release):

```bash
 while true; do sleep 1; curl -sS https://openfaas.example.com/function/env | grep HOSTNAME; done 
 
HOSTNAME=env-59bf48fb9d-cjsjw
HOSTNAME=env-59bf48fb9d-cjsjw
HOSTNAME=env-59bf48fb9d-cjsjw
HOSTNAME=env-59bf48fb9d-cjsjw
HOSTNAME=env-59bf48fb9d-cjsjw
HOSTNAME=env-59bf48fb9d-cjsjw
HOSTNAME=env-59bf48fb9d-cjsjw
HOSTNAME=env-59bf48fb9d-cjsjw
HOSTNAME=env-59bf48fb9d-cjsjw
HOSTNAME=env-canary-5dffdf4458-4vnn2
```

Access Jaeger dashboard using port forwarding:

```bash
kubectl -n istio-system port-forward deployment/istio-tracing 16686:16686 
```

Tracing the general available release:

![ga-trace](https://github.com/stefanprodan/k8s-podinfo/blob/master/docs/screens/openfaas-istio-ga-trace.png)

Tracing the canary release:

![canary-trace](https://github.com/stefanprodan/k8s-podinfo/blob/master/docs/screens/openfaas-istio-canary-trace.png)

Access Grafana using port forwarding:

```bash
kubectl -n istio-system port-forward deployment/grafana 3000:3000 
```

Monitor ga vs canary success rate and latency:

![canary-prom](https://github.com/stefanprodan/k8s-podinfo/blob/master/docs/screens/openfaas-istio-canary-prom.png)
