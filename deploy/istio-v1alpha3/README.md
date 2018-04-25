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

### Run GA and Canary Deployments

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

Create a `loadtest` pod for testing:

```bash
kubectl -n test run -i --rm --tty loadtest --image=stefanprodan/loadtest --restart=Never -- sh
```

Start the load test:

```bash
hey -n 1000000 -c 2 -q 5 http://podinfo.test:9898/version
```

**Initial state**

All traffic is routed to the GA deployment:

```yaml
apiVersion: networking.istio.io/v1alpha3
kind: VirtualService
metadata:
  name: podinfo
  namespace: test
spec:
  hosts:
  - podinfo
  - podinfo.co.uk
  gateways:
  - mesh
  - podinfo-gateway
  http:
  - route:
    - destination:
        name: podinfo.test
        subset: canary
      weight: 0
    - destination:
        name: podinfo.test
        subset: ga
      weight: 100
```

![s1](https://github.com/stefanprodan/k8s-podinfo/blob/master/docs/screens/istio-c-s1.png)

**Canary warm-up**

Route 10% of the traffic to the canary deployment:

```yaml
  http:
  - route:
    - destination:
        name: podinfo.test
        subset: canary
      weight: 10
    - destination:
        name: podinfo.test
        subset: ga
      weight: 90
```

![s2](https://github.com/stefanprodan/k8s-podinfo/blob/master/docs/screens/istio-c-s2.png)

**Canary promotion**

Increase the canary traffic to 60%:

```yaml
  http:
  - route:
    - destination:
        name: podinfo.test
        subset: canary
      weight: 60
    - destination:
        name: podinfo.test
        subset: ga
      weight: 40
```

![s3](https://github.com/stefanprodan/k8s-podinfo/blob/master/docs/screens/istio-c-s3.png)

Full promotion, 100% of the traffic to the canary:

```yaml
  http:
  - route:
    - destination:
        name: podinfo.test
        subset: canary
      weight: 100
    - destination:
        name: podinfo.test
        subset: ga
      weight: 0
```

![s4](https://github.com/stefanprodan/k8s-podinfo/blob/master/docs/screens/istio-c-s4.png)

Measure requests latency for each deployment:

![s5](https://github.com/stefanprodan/k8s-podinfo/blob/master/docs/screens/istio-c-s5.png)
 
Observe the traffic shift with Scope:

![s0](https://github.com/stefanprodan/k8s-podinfo/blob/master/docs/screens/istio-c-s0.png)
