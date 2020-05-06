#!/bin/bash

oc delete -n openshift-efs-csi \
  scc/efs-csi-scc \
  csidriver/efs.csi.aws.com \
  storageclass/efs-sc \
  serviceaccount/efs-csi-sa \
  daemonset/efs-csi-node \
  namespace/openshift-efs-csi
