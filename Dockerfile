FROM registry.svc.ci.openshift.org/openshift/release:golang-1.13 AS builder
WORKDIR /go/src/github.com/openshift/ovirt-csi-driver-operator
COPY . .
RUN make

FROM registry.svc.ci.openshift.org/openshift/origin-v4.0:base
COPY --from=builder /go/src/github.com/openshift/ovirt-csi-driver-operator/ovirt-csi-driver-operator /usr/bin/
COPY manifests /manifests

LABEL io.k8s.display-name="OpenShift ovirt-csi-driver-operator" \
      io.k8s.description="The ovirt-csi-driver-operator installs and maintains the oVirt CSI Driver on a cluster."

USER 1001 
ENTRYPOINT ["/usr/bin/ovirt-csi-driver-operator"]

