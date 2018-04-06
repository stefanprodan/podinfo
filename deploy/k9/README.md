### Cloud9 IDE Kubernetes

Create the namespaces:

```bash
kubectl apply -f ./deploy/k9/k9-ns.yaml
```

Create a secret with the Git ssh key:

```bash
kubectl apply -f ./deploy/k9/ssh-key.yaml
```

Create the Git Server deploy and service:

```bash
kubectl apply -f ./deploy/k9/git-dep.yaml
kubectl apply -f ./deploy/k9/git-svc.yaml
```

Create the Cloud9 IDE deployment:

```bash
kubectl apply -f ./deploy/k9/
```

Find the public IP:

```bash
kubectl -n ide get svc --selector=name=ide
```

Open Cloud9 IDE in your browser, login with `username/password` and run the following commands:

```bash
ssh-keyscan gitsrv >> ~/.ssh/known_hosts
git config --global user.email "user@weavedx.com" 
git config --global user.name "User"
```

Clone the repo:

```bash
git clone ssh://git@gitsrv/git-server/repos/k8s-podinfo.git
git add .
git commit -m "test"
git push origin master
```
