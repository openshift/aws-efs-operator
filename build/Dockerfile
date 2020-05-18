FROM registry.svc.ci.openshift.org/openshift/release:golang-1.13 AS builder
COPY . /go/src/github.com/openshift/aws-efs-operator
WORKDIR /go/src/github.com/openshift/aws-efs-operator
RUN make gobuild

FROM registry.svc.ci.openshift.org/openshift/origin-v4.0:base
ENV OPERATOR_BIN=aws-efs-operator

WORKDIR /root/
COPY --from=builder /go/src/github.com/openshift/aws-efs-operator/build/_output/bin/${OPERATOR_BIN} /usr/local/bin/${OPERATOR_BIN}
LABEL io.openshift.managed.name="aws-efs-operator" \
      io.openshift.managed.description="This operator enables creation of AWS EFS-backed read-write-many persistent volumes."
