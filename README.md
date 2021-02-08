# An operator to deploy oVirt's CSI driver

This operator is meant to be installed and controlled by the [Cluster Storage Operator](https://github.com/openshift/cluster-storage-operator)

Container Image: https://quay.io/repository/ovirt/csi-driver-operator

This operator will deploy and watch oVirt csi driver components:
- __OvirtCSIOperator__ - the main operator object  
- __StatefulSet__ of the csi controller
- __DaemonSet__ of the csi node
- RBAC objects (__ServiceAccount__, __ClusterRoles__, __RoleBindings__)
      
## Manual Installation

1. The storage class is created automatically by the operator and the used ovirt storage domain is taken from the installation storage domain for openshift. So if this is not the desired ovirt storage domain (e.g. because for openshift you used storage domain "vmstore" and for the data of openshift workloads you want to use a storage domain named "data"), you have to create the storage class before installing the operator in the following way:
```bash
cat << EOF | oc create -f -
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: ovirt-csi-sc
  annotations:
    storageclass.kubernetes.io/is-default-class: "true"
provisioner: csi.ovirt.org
parameters:
  storageDomainName: "YOUR-STORAGE-DOMAIN"
  thinProvisioning: "true"
reclaimPolicy: Delete
allowVolumeExpansion: false
volumeBindingMode: Immediate
EOF
```

2. Deploy the operator from [manifests/](manifests) directory(needs [jq tool](https://stedolan.github.io/jq/)):
```bash
curl -s https://api.github.com/repos/openshift/ovirt-csi-driver-operator/contents/manifests \
 | jq '.[].download_url' \
 | xargs curl -Ls \
 | oc create -f -
```
## Development

- everyday standard 
```bash
make build verify
```

- create a container image tagged `quay.io/ovirt/ovirt-csi-driver-operator:latest`
```bash
make image
```
