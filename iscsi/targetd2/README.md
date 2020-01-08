# iSCSI-targetd provisioner2

iSCSI-targetd provisioner2 is an out of tree provisioner for iSCSI storage for
Kubernetes and OpenShift.  The provisioniner provides iscsi server and uses 
targetcli to create and export iSCSI storage.

## Run this provisioner in a k8s/openshift cluster
```
oc create ns iscsi-provisioner
oc create -f deploy/iscsi-rbac.yaml
oc adm policy add-scc-to-user privileged system:serviceaccount:iscsi-provisioner:iscsi-provisioner
oc create -f deploy/iscsi-provisioner-class.yaml
oc create -f deploy/iscsi-provisioner-dc.yaml
```

## Run this provisioner out of the cluster
Suppose we run provisioner in the bootstrap node
```
oc create -f deploy/iscsi-provisioner-class.yam
mkdir /mnt/iscsi_disks
mkdir /mnt/kube
cp <kubeconfig> to /mnt/kube
podman run -d --rm --name iscsi-provisioner --privileged --network host -v /lib/modules:/lib/modules -v /mnt/kube:/kube -v /mnt/iscsi_disks:/iscsi_disks -e TARGET_IP=10.0.149.55 -e TARGET_PORT=3260 docker.io/aosqe/iscsi-provisioner start --kubeconfig=/kube/kubeconfig
```

## What't the different with the iSCSI-targetd provisioner
1. Do not need to prepare iSCSI server seperatly
2. Use targetcli to create and export iSCSI storage
3. Do not support multi-path
4. Do not support any authentication, so do not need to do any config in iSCSI initiator end

## Test the iSCSI-targetd provisioner2
1. Filesystem PV
```
oc create -f deploy/iscsi-provisioner-pvc.yaml
oc create -f deploy/iscsi-test-pod.yaml
```

2. Block PV
```
oc create -f deploy/iscsi-provisioner-pvc-block.yaml
oc create -f deploy/iscsi-test-pod-block.yaml
```
