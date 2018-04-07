### Cloud9 IDE Kubernetes

Create the namespaces:

```bash
kubectl apply -f ./deploy/k9/k9-ns.yaml
```

Create a secret with the Git ssh key:

```bash
kubectl apply -f ./deploy/k9/ssh-key.yaml
```

Create the Git Server deployment and service:

```bash
kubectl apply -f ./deploy/k9/git-dep.yaml
kubectl apply -f ./deploy/k9/git-svc.yaml
```

Deploy Flux (modify fux-dep.yaml and add your weave token):

```bash
kubectl apply -f ./deploy/k9/memcache-dep.yaml
kubectl apply -f ./deploy/k9/memcache-svc.yaml
kubectl apply -f ./deploy/k9/flux-rbac.yaml
kubectl apply -f ./deploy/k9/flux-dep.yaml
```

Create the Cloud9 IDE deployment:

```bash
kubectl apply -f ./deploy/k9/
```

Find the public IP:

```bash
kubectl -n ide get svc --selector=name=ide
```

Open Cloud9 IDE in your browser, login with `username/password` and config git:

```bash
git config --global user.email "user@weavedx.com" 
git config --global user.name "User"
```

Commit a change to podinfo repo:

```bash
cd k8s-podinfo
rm Dockerfile.build
git add .
git commit -m "test"
git push origin master
```


