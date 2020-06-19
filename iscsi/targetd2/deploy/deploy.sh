#!/bin/sh

ns="iscsi-provisioner"
oc create ns ${ns}
oc create -f iscsi-rbac.yaml
oc create -f service.yaml
sleep 3
oc adm policy add-scc-to-user privileged system:serviceaccount:iscsi-provisioner:iscsi-provisioner
target=`oc -n ${ns} get svc -l app=iscsi-tgt -o custom-columns=Clusterip:.spec.clusterIP,Image:.spec.ports[].port --no-headers | awk '{print $1":"$2}'|paste -sd ","`
sed "s/#TARGET#/${target}/g" iscsi-provisioner-statefulset.yaml |oc create -f -

oc create -f iscsi-provisioner-class.yaml
