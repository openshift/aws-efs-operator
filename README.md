# AWS EFS Operator for OpenShift Dedicated

- [AWS EFS Operator for OpenShift Dedicated](#aws-efs-operator-for-openshift-dedicated)
  - [Overview](#overview)
  - [Installing](#installing)
  - [Usage](#usage)
    - [AWS EFS and Access Points](#aws-efs-and-access-points)
    - [Working with `SharedVolume` resources](#working-with-sharedvolume-resources)
      - [Create a `SharedVolume`.](#create-a-sharedvolume)
      - [Monitor the `SharedVolume`.](#monitor-the-sharedvolume)
      - [Check the `PersistentVolumeClaim`.](#check-the-persistentvolumeclaim)
      - [Create Pod(s).](#create-pods)
      - [Validate access.](#validate-access)
      - [Cleaning up](#cleaning-up)
  - [Uninstalling](#uninstalling)
  - [Troubleshooting](#troubleshooting)
  - [Limitations, Caveats, Known Issues](#limitations-caveats-known-issues)
    - [Size doesn't matter](#size-doesnt-matter)
    - [Don't edit `SharedVolume`s](#dont-edit-sharedvolumes)
    - [Don't mess with generated `PersistentVolumeClaim`s (or `PersistentVolume`s)](#dont-mess-with-generated-persistentvolumeclaims-or-persistentvolumes)
  - [Under the hood](#under-the-hood)

This is an operator to manage read-write-many access to AWS EFS volumes in an OpenShift Dedicated cluster.

## Overview
The operator watches for instances of a custom resource called `SharedVolume`.
One `SharedVolume` enables mounting an EFS access point by creating a
`PersistentVolumeClaim` you can use in a `volume` definition in a pod. Such mounts are `ReadWriteMany` --
i.e. assuming proper ownership and permissions, the contents are readable and writable by multiple containers,
in different pods, on different worker nodes, in different namespaces or availability zones.

Pods in the *same namespace* can use the same `SharedVolume`'s `PersistentVolumeClaim` to mount the same access point.

A `SharedVolume` specifying the *same access point* in a *different* namespace can be created to enable mounting
the same access point by pods in different namespaces.

You can create `SharedVolume`s specifying *different access points* to create distinct data stores.

## Installing
This operator is available via OperatorHub.
More detailed information can be found [here](https://access.redhat.com/articles/5025181).

## Usage

### AWS EFS and Access Points
(A detailed discussion of EFS is beyond the scope of this document.)

Create an [EFS file system](https://docs.aws.amazon.com/efs/latest/ug/gs-step-two-create-efs-resources.html),
configured appropriately with respect to VPC, availability zones, etc.

Create a separate [access point](https://docs.aws.amazon.com/efs/latest/ug/create-access-point.html) for each
distinct data store you wish to access from your cluster. Be sure to configure ownership and permissions that
will allow read and/or write access by your pod's `uid`/`gid` as desired.

Access points need not be backed by separate EFS file systems.

### Working with `SharedVolume` resources

#### Create a `SharedVolume`.

This operator's custom resource, `SharedVolume` (which can be abbreviated `sv`) requires two pieces of information:
- The ID of the EFS file system, which will look something like `fs-1234cdef`.
- The ID of the Access Point, which will look something like `fsap-0123456789abcdef`.

Here is an example `SharedVolume` definition:

```yaml
apiVersion: aws-efs.managed.openshift.io/v1alpha1
kind: SharedVolume
metadata:
  name: sv1
spec:
  accessPointID: fsap-0123456789abcdef
  fileSystemID: fs-1234cdef
```

If the above definition is in the file `/tmp/sv1.yaml`, create the resource with the command:

```shell
$ oc create -f /tmp/sv1.yaml
sharedvolume.aws-efs.managed.openshift.io/sv1 created
```

Note that a `SharedVolume` is namespace scoped. Create it in the same namespace in which you wish to run the
pods that will use it.

#### Monitor the `SharedVolume`.

Watch the `SharedVolume` using `oc get`:

```shell
$ oc get sv sv1
NAME   FILE SYSTEM   ACCESS POINT             PHASE    CLAIM     MESSAGE
sv1    fs-1234cdef   fsap-0123456789abcdef    Pending
```

When the operator has finished its work, the `PHASE` will become `Ready` and a name will appear in the `CLAIM` column:

```shell
$ oc get sv sv1
NAME   FILE SYSTEM   ACCESS POINT             PHASE   CLAIM     MESSAGE
sv1    fs-1234cdef   fsap-0123456789abcdef    Ready   pvc-sv1   
```

#### Check the `PersistentVolumeClaim`.

The `CLAIM` is the name of a `PersistentVolumeClaim` created by the operator in the same namespace as the `SharedVolume`.
Validate that the `PersisentVolumeClaim` is ready for use by ensuring it is `Bound`:

```shell
$ oc get pvc pvc-sv1
NAME      STATUS   VOLUME         CAPACITY   ACCESS MODES   STORAGECLASS   AGE
pvc-sv1   Bound    pv-proj2-sv1   1          RWX            efs-sc         23s
```

#### Create Pod(s).

Use the `PersistentVolumeClaim` in a pod's `volume` definition. For example:

```yaml
kind: Pod
metadata:
  name: pod1
spec:
  volumes:
    - name: efsap1
      persistentVolumeClaim:
        claimName: pvc-sv1
  containers:
    - name: test-efs-pod
      image: centos:latest
      command: [ "/bin/bash", "-c", "--" ]
      args: [ "while true; do sleep 30; done;" ]
      volumeMounts:
        - mountPath: /mnt/efs-data
          name: efsap1
```

```shell
$ oc create -f /tmp/pod1.yaml
pod/pod1 created
```

#### Validate access.

Within the pod's container, you should see the specified `mountPath` with the ownership and permissions you
used when you created the access point in AWS.
This should allow read and/or write access with normal POSIX semantics.

```shell
$ oc rsh pod1
sh-4.4$ cd /mnt/efs-data
sh-4.4$ ls -lFd .
drwxrwxr-x. 2 1000123456 root 6144 May 14 16:47 ./
sh-4.4$ echo "Hello world" > f1
sh-4.4$ cat f1
Hello world
```

#### Cleaning up

Once all pods using a `SharedVolume` have been destroyed, delete the `SharedVolume`:

```shell
$ oc delete sv sv1
sharedvolume.aws-efs.managed.openshift.io "sv1" deleted
```

The associated `PersistentVolumeClaim` is deleted automatically.

Note that the data in the EFS file system persists even if all associated `SharedVolume`s have been deleted.
A new `SharedVolume` to the same access point will reveal that same data to attached pods.

## Uninstalling
Uninstalling currently requires the following steps:

1. Delete all workloads using `PersistentVolumeClaim`s generated by the operator.
2. Remove all instances of the `SharedVolume` CR from all namespaces. The operator will automatically remove the associated PVs and PVCs.
3. Uninstall the operator via OCM:
   * Navigate to Operators => Installed Operators.
   * Find and click "AWS EFS Operator".
   * Click Actions => Uninstall Operator.
   * Click "Uninstall".
4. Delete the SharedVolume CRD. This will trigger deletion of the remaining operator-owned resources. This must be done as `cluster-admin`:

```
      $ oc delete -n crd/sharedvolumes.aws-efs.managed.openshift.io
```

## Troubleshooting
If you uninstall the operator while `SharedVolume` resources still exist, attempting to delete the CRD or `SharedVolume` CRs will hang on finalizers.
In this state, attempting to delete workloads using `PersistentVolumeClaim`s associated with the operator will also hang.
If this happens, reinstall the operator, which will reconcile the current state appropriately and allow any pending deletions to complete.
Then perform the [uninstallation](#uninstalling) steps in order.

## Limitations, Caveats, Known Issues

### Size doesn't matter

You may notice that the `PersistentVolumeClaim` (and its associated `PersistentVolume`) created at the behest of your
`SharedVolume` has a `CAPACITY` value. This is meaningless.
The backing file system is elastic (hence the name) and grows as needed to a
[maximum of 47.9TiB](https://github.com/awsdocs/amazon-efs-user-guide/blob/master/doc_source/limits.md#limits-for-amazon-efs-file-systems)
unless it hits some other limit (e.g. a quota) first.
However, the kubernetes APIs for `PersistentVolume` and `PersistentVolumeClaim` require that a value be specified.
The number we chose is arbitrary.

### Don't edit `SharedVolume`s

You can't switch out an access point or file system identifier in flight.
If you need to connect your pod to a different access point, create a new `SharedVolume`.
If you no longer need the old one, delete it.

We feel strongly enough about this that the operator is designed to try to "un-edit" your `SharedVolume` if it
detects a change.

### Don't mess with generated `PersistentVolumeClaim`s (or `PersistentVolume`s)

`PersistentVolumeClaim`s are normally under the user's purview.
However, deleting or modifying the `PersistentVolumeClaim` (or `PersistentVolume`) associated with a `SharedVolume`
can leave it in an unusable state, even if the operator is able to resurrect the resources themselves.

The only supported way to delete a `PersistentVolumeClaim` (or `PersistentVolume`) associated with a `SharedVolume`
is to delete the `SharedVolume` and let the operator do the rest.

## Under the hood

The operator has two controllers. One monitors the resources necessary to run the
[AWS EFS CSI driver](https://github.com/kubernetes-sigs/aws-efs-csi-driver).
These are set up once and should never change, except on operator upgrade.

The other controller is responsible for `SharedVolume` resources.
It monitors all namespaces, allowing `SharedVolume`s to be created in any namespace.
It creates a `PersistentVolume`/`PersistentVolumeClaim` pair for each `SharedVolume`.
Though the `PersistentVolume` is technically cluster-scoped, it is inextricably bound to its
`PersistentVolumeClaim`, which is namespace-scoped.

See the [design](DESIGN.md) for more.
