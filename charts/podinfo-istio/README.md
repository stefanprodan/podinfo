# Podinfo Istio

Podinfo is a tiny web application made with Go 
that showcases best practices of running microservices in Kubernetes.

## Installing the Chart

Create an Istio enabled namespace:

```console
kubectl create namespace demo
kubectl label namespace demo istio-injection=enabled
```

Create an Istio Gateway in the `istio-system` namespace named `public-gateway`:

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

Create the `frontend` release by specifying the external domain name:

```console
helm upgrade frontend --install ./charts/podinfo-istio \
  --namespace=demo \
  --set host=podinfo.example.com \
  --set gateway.name=public-gateway \
  --set gateway.create=false \
  -f ./charts/podinfo-istio/frontend.yaml
```

Create the `backend` release:

```console
helm upgrade backend --install ./charts/podinfo-istio \
  --namespace=demo \
  -f ./charts/podinfo-istio/backend.yaml 
```

Create the `store` release:

```console
helm upgrade store --install ./charts/podinfo-istio \
  --namespace=demo \
  -f ./charts/podinfo-istio/store.yaml 
```




