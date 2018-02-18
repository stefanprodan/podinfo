# StatefulSets with local storage PV

Running StatefulSet with with local persistent volumes for bare-metal Kubernetes 1.9 clusters.

### Cluster provisioning

I'm assuming you have tree hosts:

* kube-master-0 
* kube-node-0
* kube-node-1

In order to use local PVs the Kubernetes API Server, controller-manager, scheduler must be 
configured with a series of `FEATURE_GATES`. 

On `kube-master-0` machine save the following config as `master.yaml`:

```yaml
apiVersion: kubeadm.k8s.io/v1alpha1
kind: MasterConfiguration
api:
  advertiseAddress: #privateip#
networking:
  podSubnet: "10.32.0.0/12" # default Weave Net IP range
apiServerExtraArgs:
  service-node-port-range: 80-32767
  feature-gates: "PersistentLocalVolumes=true,VolumeScheduling=true,MountPropagation=true"
controllerManagerExtraArgs:
  feature-gates: "PersistentLocalVolumes=true,VolumeScheduling=true,MountPropagation=true"
schedulerExtraArgs:
  feature-gates: "PersistentLocalVolumes=true,VolumeScheduling=true,MountPropagation=true"
```

Replace `#privateip#` with the private IP of `kube-master-0` and initialize the Kubernetes master with:

```bash
kubeadm init --config ./master.yaml
```

Run the kubeadm join command on `kube-node-0` and `kube-node-1`. 

Add the `role` label to the worker nodes:

```bash
kubectl --kubeconfig ./admin.conf label nodes kube-node-0 role=local-ssd
kubectl --kubeconfig ./admin.conf label nodes kube-node-0 role=local-ssd
```

### Persistent volumes provisioning

Create a Storage Class that will delay volume binding until pod scheduling:

```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: local-ssd
provisioner: kubernetes.io/no-provisioner
volumeBindingMode: WaitForFirstConsumer
```

Save the definition as `storage-class.yaml` and apply it:

```yaml
kubectl apply -f ./storage-class.yaml
```

On each worker node create the following dir:

```bash
mkdir -p /mnt/data
```

Create the Persistent Volume definition for `kube-node-0`:

```yaml
apiVersion: v1
kind: PersistentVolume
metadata:
  name: podinfo-vol-0
  annotations:
    "volume.alpha.kubernetes.io/node-affinity": '{
      "requiredDuringSchedulingIgnoredDuringExecution": {
        "nodeSelectorTerms": [
          { "matchExpressions": [
              { "key": "kubernetes.io/hostname",
                "operator": "In",
                "values": ["kube-node-0"]
              }
          ]}
        ]}}'
spec:
  capacity:
    storage: 1Gi
  accessModes:
  - ReadWriteOnce
  persistentVolumeReclaimPolicy: Retain
  storageClassName: local-ssd
  local:
    path: /mnt/data
```

Do the same for the second node by changing the PV name to `podinfo-vol-1` and the 
node selector expression to `kube-node-1`.

Save the PVs files as `pv-0.yaml` and `pv-1.yaml` and apply them with:

```yaml
kubectl apply -f ./pv-0.yaml,pv-1.yaml
``` 

### StatefulSet config

Create a StatefulSet definition with two replicas:

```yaml
apiVersion: apps/v1beta2
kind: StatefulSet
metadata:
  name: podinfo
spec:
  serviceName: "data"
  replicas: 2
  podManagementPolicy: OrderedReady
  selector:
    matchLabels:
      app: podinfo
  template:
    metadata:
      labels:
        app: podinfo
      annotations:
        prometheus.io/scrape: "true"
    spec:
      affinity:
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
            - matchExpressions:
              - key: role
                operator: In
                values:
                - local-ssd
        podAntiAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
          - labelSelector:
              matchExpressions:
              - key: app
                operator: In
                values:
                - podinfo
            topologyKey: "kubernetes.io/hostname"
      containers:
        - name: podinfod
          image: stefanprodan/podinfo:0.0.7
          command:
            - ./podinfo
            - -port=9898
            - -logtostderr=true
            - -v=2
          ports:
            - name: http
              containerPort: 9898
              protocol: TCP
          volumeMounts:
          - name: data
            mountPath: /data
  volumeClaimTemplates:
  - metadata:
      name: data
    spec:
      accessModes: [ "ReadWriteOnce" ]
      storageClassName: local-ssd
      resources:
        requests:
          storage: 1Gi
```

The node affinity spec instructs the scheduler to create the StatefulSet prods only on the 
nodes that are labeled with `role=local-ssd`. The pod anti-affinity spec prohibits the scheduler 
to create more than one pod per node.

The volume claim template will create a PVC on each node targeting volumes with the `local-ssd` 
storage class.

Save the above definition as `statefulset.yaml` and apply it:

```bash
kubectl apply -f ./statefulset.yaml
```

Once the podinfo StatefulSet has been deployed you can check if the volumes have been claimed with:

```bash
kubectl get pvc
NAME              STATUS    VOLUME          CAPACITY   ACCESS MODES   STORAGECLASS    AGE
data-prodinfo-0   Bound     podinfo-vol-0   1Gi        RWO            local-ssd       7h
data-prodinfo-1   Bound     podinfo-vol-1   1Gi        RWO            local-ssd       7h
```

Create a headless service to expose the podinfo StatefulSet:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: podinfo
spec:
  clusterIP: None
  publishNotReadyAddresses: false
  ports:
  - port: 9898
    targetPort: 9898
  selector:
    app: podinfo
```

Save the above definition as `service.yaml` and apply it:

```bash
kubectl apply -f ./service.yaml
```

Each podinfo replica has its own DNS address as in <pod-name>.<service-name>.<namespace>. 

Create a temporary curl pod in the default namespace in order to access the StatefulSet:

```yaml
kubectl run -i --rm --tty curl --image=radial/busyboxplus:curl --restart=Never -- sh
```

Inside the curl container issue a write command for `podinfo-0`:

```bash
[ root@curl:/ ]$ curl -d 'test' storage-probe-0.storage-probe:9898/write
74657374da39a3ee5e6b4b0d3255bfef95601890afd80709
```

Now read the file using the SHA1 hash:

```bash
[ root@curl:/ ]$ curl -d '74657374da39a3ee5e6b4b0d3255bfef95601890afd80709' storage-probe-0.storage-probe:9898/read
test
```

You can remove the StatefulSet, PV and service with:

```yaml
kubectl delete -f ./service.yaml,statefulset.yaml,pv-0.yaml,pv-1.yaml
```

The Persistent Volumes Claims can be remove with:

```yaml
kubectl delete pvc data-prodinfo-0 data-prodinfo-1
```

