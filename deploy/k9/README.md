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

Open Cloud9 IDE in your browser and run the following commands:

```bash
ssh-keyscan gitsrv >> ~/.ssh/known_hosts
git config --global user.email "user@weavedx.com" 
git config --global user.name "User"
```

Exec into the Git server and create a repo:

```bash
kubectl -n ide exec -it gitsrv-69b4cd5fc-dd6rf -- sh

/git-server # cd repos
/git-server # mkdir myrepo.git
/git-server # cd myrepo.git
/git-server # git init --shared=true
/git-server # git add .
/git-server # git config --global user.email "user@weavedx.com" 
/git-server # git config --global user.name "User"
/git-server # git commit -m "init"
/git-server # git checkout -b dummy
```

Go back to the Cloud9 IDE and clone the repo:

```bash
git clone ssh://git@gitsrv/git-server/repos/myrepo.git
git add .
git commit -m "test"
git push origin master
```
