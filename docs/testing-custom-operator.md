# Creating a custom oVirt CSI driver

When we develop the oVirt CSI driver operator it's usually helpful to test it against a running cluster.
The following doc explains how to do it.

## Prerequisits

- Ready OpenShift cluster running on oVirt and a user with admin permissions.
- Container registry (recommended quay.io) account which the OpenShift cluster can reach and pull from.

Note:
For info about installing and launching a new cluster using oVirt check:

- [Installer Provisioned Infrastructure (IPI)](https://github.com/openshift/installer/blob/master/docs/user/ovirt/install_ipi.md)
- [User Provisioned Infrastructure (UPI)](https://github.com/openshift/installer/blob/master/docs/user/ovirt/install_upi.md)

## Step 1: Create a custome container image

```bash
## Logic to container registry
$ podman login quay.io
$ podman build -t quay.io/${username}/ovirt-csi-driver-operator:${tag} .
$ podman push quay.io/${username}/ovirt-csi-driver-operator:${tag}
```

- ${username} - container registry username
- ${tag} - ovirt-csi-driver-operator container image tag.

** In this example I used quay.io as my container registry, but you can use any registry as long as the cluster will be
able to pull from it (easiest is to set it to public).

## Step 2: Stop Cluster Version Operator

In the running cluster cluster-version-operator is responsible for maintaining functioning and
non-altered elements.

In our case we need to be able to use a custom image, so we will need to disable the Cluster Version Operator
that it will not detect that we changed the image and replace it back to the original image, the easiest way
to do it is to scale down the Cluster Version Operator:

```bash
$ oc scale --replicas=0 deploy/cluster-version-operator -n openshift-cluster-version
```

**Note:**

This version disables the Cluster Version Operator and it is recommended for testing purposes only, once you are
done with the test you should scale the operator back

## Step 3: Replacing the container image

The csi operator and driver are both managed by the storage operator, in order to replace the driver image you will
need to make the storage operator to use the custom image you created.

To do so first edit the cluster-storage-operator deployment:

```bash
 oc -n openshift-cluster-storage-operator edit  deployment.apps/cluster-storage-operator
```

Then change the environment variable "OVIRT_DRIVER_OPERATOR_IMAGE" (which is found under spec.template.spec.containers[0].env 
 in the deployment yaml) to the with the pull (by digest) of your image.
 
 For example:
 ```yaml
##  Change:
         - name: OVIRT_DRIVER_OPERATOR_IMAGE
           value: quay.io/openshift-release-dev/ocp-v4.0-art-dev@sha256:6833ab166bae5e85ea627db0083d9ec4978a5cc2d5d92a843e31a54116aaa2e7
## To:
        - name: OVIRT_DRIVER_OPERATOR_IMAGE
          value: quay.io/gzaidman/ovirt-csi-driver-operator@sha256:2aeb0886fafca7b46981a8b2fe8490f1b534b3546a40d48b9ab094dd1287513e
```

This will cause the operator pod in the openshift-cluster-csi-drivers namespace to restart and change the image

NOTE:
To find the digest on quay.io, go to your image page and under tags click on fetch tag by digest  

## Step 4: verify container image

Verify that the process was successful

```bash
// Check that all the pods have restarted
  oc -n openshift-cluster-csi-drivers get pod ovirt-csi-driver-operator-XXXXXXXX

// Check pods contain the correct image
  oc -n openshift-cluster-csi-drivers get pod/ovirt-csi-driver-operator-XXXXXXXX -ojson|jq '.spec.containers[]|select(.name == "ovirt-csi-driver-operator")|."image"'
```
