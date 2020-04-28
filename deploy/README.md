# Deploy demo webapp 

Demo webapp manifests:
- [common](webapp/common)
- [frontend](webapp/frontend)
- [backend](webapp/backend)

Deploy the demo in `webapp` namespace:

```bash
kubectl apply -f ./webapp/common
kubectl apply -f ./webapp/backend
kubectl apply -f ./webapp/frontend
```

Deploy the demo in the `dev` namespace:

```bash
kustomize build ./overlays/dev | kubectl apply -f-
```

Deploy the demo in the `staging` namespace:

```bash
kustomize build ./overlays/staging | kubectl apply -f-
```

Deploy the demo in the `production` namespace:

```bash
kustomize build ./overlays/production | kubectl apply -f-
```
