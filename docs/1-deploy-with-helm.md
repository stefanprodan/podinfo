# Deploy 

Deploy and upgrade guide

### Setup Helm 

Install Helm CLI:

```bash
brew install kubernetes-helm
```

Create a service account for Tiller:

```bash
kubectl -n kube-system create sa tiller
```

Create a cluster role binding for Tiller:

```bash
kubectl create clusterrolebinding tiller-cluster-rule \
    --clusterrole=cluster-admin \
    --serviceaccount=kube-system:tiller 
```

Deploy Tiller in kube-system namespace:

```bash
helm init --skip-refresh --upgrade --service-account tiller
```

### Using the Helm chart

Create a namespace:

```bash
kubectl create namespace test
```

Create a release named frontend:

```bash
helm upgrade --install --wait frontend \
    --set service.type=NodePort \
    --set service.nodePort=30098 \
    --namespace test \
    ./chart/stable/podinfo
```

Check if frontend is accessible from within the cluster:

```bash
helm test --cleanup frontend
```

Check if the frontend is available from outside the cluster:

```bash
curl http://<KUBE_PUBLIC_IP>:30098/version
```

Set CPU/memory requests and limits:

```bash
helm upgrade --reuse-values frontend \
    --set resources.requests.cpu=10m \
    --set resources.limits.cpu=100m \
    --set resources.requests.memory=16Mi \
    --set resources.limits.memory=128Mi \
    ./chart/stable/podinfo
```

Setup horizontal pod autoscaling (HPA) based on CPU average usage and memory consumption:

```bash
helm upgrade --reuse-values frontend \
    --set hpa.enabled=true \
    --set hpa.maxReplicas=10 \
    --set hpa.cpu=80 \
    --set hpa.memory=200Mi \
    ./chart/stable/podinfo
```

Increase the minimum replica count:

```bash
helm upgrade --reuse-values frontend \
    --set replicaCount=2 \
    ./chart/stable/podinfo
```

Update podinfo version:

```bash
helm upgrade --reuse-values frontend \
    --set image.tag=0.0.6 \
    ./chart/stable/podinfo
```

Rollback to the previous version:

```bash
helm rollback frontend
```

Delete the release:

```bash
helm delete --purge frontend
```

