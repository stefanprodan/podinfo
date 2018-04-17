# Istio

### Install

Download latest release:

```bash
curl -L https://git.io/getLatestIstio | sh -
```

Add the istioctl client to your PATH:

```bash
cd istio-0.7.1
export PATH=$PWD/bin:$PATH
```

Install Istio services without enabling mutual TLS authentication:

```bash
kubectl apply -f install/kubernetes/istio.yaml
``` 

### Setup automatic sidecar injection

Generate certs:

```bash
./install/kubernetes/webhook-create-signed-cert.sh \
    --service istio-sidecar-injector \
    --namespace istio-system \
    --secret sidecar-injector-certs
```

Install the sidecar injection configmap:

```bash
kubectl apply -f install/kubernetes/istio-sidecar-injector-configmap-release.yaml
```

Set the caBundle in the webhook install YAML that the Kubernetes api-server uses to invoke the webhook:

```bash
cat install/kubernetes/istio-sidecar-injector.yaml | \
     ./install/kubernetes/webhook-patch-ca-bundle.sh > \
     install/kubernetes/istio-sidecar-injector-with-ca-bundle.yaml
```

Install the sidecar injector webhook:

```bash
kubectl apply -f install/kubernetes/istio-sidecar-injector-with-ca-bundle.yaml
```

Create the `test` namespace:

```bash
kubectl create namespace test
```

Label the `test` namespace with `istio-injection=enabled`:

```bash
kubectl label namespace test istio-injection=enabled
```

### Run canary deployment

Apply the podinfo ga and canary deployments and service:

```bash
kubectl -n test apply -f ./deploy/istio-v1alpha3/ga-dep.yaml,./deploy/istio-v1alpha3/canary-dep.yaml,./deploy/istio-v1alpha3/svc.yaml
```

Apply the istio destination rule, virtual service and gateway:

```bash
kubectl -n test apply -f ./deploy/istio-v1alpha3/istio-destination-rule.yaml
kubectl -n test apply -f ./deploy/istio-v1alpha3/istio-virtual-service.yaml
kubectl -n test apply -f ./deploy/istio-v1alpha3/istio-gateway.yaml
```

Create a `curl` pod for testing:

```bash
kubectl -n test run -i --rm --tty curl --image=radial/busyboxplus:curl --restart=Never -- sh
```

Run inside the `curl` pod:

```bash
curl -v -H "Host: podinfo.test" http://podinfo.test:9898/version
version: 0.2.1

curl -v -H "x-user: insider" -H "Host: podinfo.test" http://podinfo.test:9898/version
version: 0.2.2
```
