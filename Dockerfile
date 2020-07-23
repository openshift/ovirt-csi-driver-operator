FROM registry.svc.ci.openshift.org/openshift/release:golang-1.13 AS builder

WORKDIR /src/ovirt-csi-driver-operator
COPY . .
RUN make build

FROM registry.svc.ci.openshift.org/openshift/origin-v4.0:base

COPY --from=builder /src/ovirt-csi-driver-operator/ovirt-csi-driver-operator /usr/local/bin/
COPY --from=builder /src/ovirt-csi-driver-operator/manifests /manifests

LABEL io.k8s.display-name="OpenShift ovirt-csi-driver-operator" \
      io.k8s.description="The ovirt-csi-driver-operator installs and maintains the oVirt CSI Driver on a cluster."

USER 1001 
ENTRYPOINT ["ovirt-csi-driver-operator"]

