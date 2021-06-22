# An operator to deploy oVirt's CSI driver

> __UNDER DEVELOPMENT__

Container Image: https://quay.io/repository/ovirt/csi-driver-operator

This operator will deploy and watch oVirt csi driver components:
- __OvirtCSIOperator__ - the main operator object  
- __StatefulSet__ of the csi controller
- __DaemonSet__ of the csi node
- RBAC objects (__ServiceAccount__, __ClusterRoles__, __RoleBindings__)
      
## Installation

1. Deploy the operator from [manifests/](manifests) directory(needs [jq tool](https://stedolan.github.io/jq/)):
```bash
curl -s https://api.github.com/repos/ovirt/csi-driver-operator/contents/manifests \
 | jq '.[].download_url' \
 | xargs curl -L \
 | oc create -f -

```
2. Create a storage class and point it to the oVirt storage domain in use:
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
EOF
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

- To test your modified CSI driver operator on a test cluster follow [Docs](docs/testing-custom-operator.md)
