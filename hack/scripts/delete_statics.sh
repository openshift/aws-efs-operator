#!/bin/bash

oc delete \
  scc/efs-csi-scc \
  csidriver/efs.csi.aws.com \
  storageclass/efs-sc \
  serviceaccount/efs-csi-sa \
  daemonset/efs-csi-node
