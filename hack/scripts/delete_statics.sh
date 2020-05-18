#!/bin/bash

oc delete \
  scc/efs-csi-scc \
  csidriver/efs.csi.aws.com \
  storageclass/efs-sc \
  serviceaccount/aws-efs-operator \
  daemonset/efs-csi-node
