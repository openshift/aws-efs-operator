#!/bin/bash

oc delete \
  scc/aws-efs-scc \
  csidriver/efs.csi.aws.com \
  storageclass/efs-sc \
  serviceaccount/aws-efs-sa \
  daemonset/aws-efs-node
