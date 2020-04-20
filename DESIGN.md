# Design

## Objectives
- Access to a shared writable file system
- from multiple pods
- even when those pods are on different worker nodes, and
- even when those nodes are in different availability zones.
- Do not require the customer to possess any special permissions, roles, etc.

## Environment
This operator supports:
- **OpenShift Dedicated** with
- **BYOC clusters** backed by
- **AWS**.
- **Shared storage** backed by
- **EFS volumes**, by way of
- The **CSI driver**.

## Artifacts

### AWS
In the current version, it is the customer's responsibility to create and maintain the necessary artifacts in AWS, per the
instructions in [this document](https://docs.google.com/document/d/1KdcqZirAdjZ2mJeqOKiMqiNTz4_VVZf7ePD5aB9RVXk).

This operator will require as input:
- The EFS volume's file system ID.
- The ID of a file system access point (this allows the customer to control ownership/permissions of the NFS mount).

### Per Cluster
At the cluster level, the operator must create and maintain:
- A `CSIDriver` resource.
- A `DaemonSet` running the driver image.
    - This is restricted to running on worker nodes.
- A `Namespace` for the `DaemonSet` with a `ServiceAccount` and `SecurityContextConstraints` giving the
  `DaemonSet` the power to manipulate the paths and network resources necessary to make the driver function.
- A `StorageClass`.

We only need one instance of these artifacts per cluster, and their configuration will not change.
(It will in fact be identical from cluster to cluster, so the implementation may wish to maintain these
resource definitions in a textual format such as YAML, rather than modeled in go.)

### Per Namespace
A pod can only use a PersistentVolumeClaim in its namespace.
The PersistentVolume associated with the EFS CSI driver can only be bound to one PersistentVolumeClaim.
Thus, by extension, we must have one PersistentVolume per namespace, despite the fact that a
PersistentVolume is not inherently namespace-scoped.

The current version only supports a single PV/PVC/namespace.
So at the moment this reduces to the operator simply creating and managing a single PV.

## Custom Resource Definition
This operator's Custom Resource shall be named **SharedVolume**.

Its namespace shall be `efs-csi-operator`.

Its spec shall contain:

| json            | go            | type   | required? | description |
|-|-|-|-|-|
| `fileSystemID`  | FileSystemID  | string | y         | The EFS volume identifier (e.g. `fs-484648c8`) |
| `accessPointID` | AccessPointID | string | n         | The access point identifier (e.g. `fsap-097bd0daaba932e64`) |
| `useTLS`        | UseTLS        | bool   | n         | Whether to encrypt data in transit |
||||||

Notes:
- If `accessPointID` is omitted, the volume will be mounted against the root of the NFS export.
  By default, this has root user and group ownership and 755 permissions, which customer pods by default will
  not be able to access.
- `useTLS` will default to `true`, but cannot be set to `false` if using an `accessPointID`.
- Because the current version only has support for a single EFS volume, the operator shall refuse to
  create more than one SharedVolume CR.
- Due to the complications of managing PVC bindings, the current version will also refuse to allow the
  `fileSystemID` or `accessPointID` to be edited once set. In order to switch to another file system, or switch
  to/from an access point, or change TLS options, the SharedVolume must be deleted and a new one created.

## Initialization
On startup, the operator will create all the [per-cluster resources](#per-cluster).

## Reconciliation
The reconciliation loop will ensure (create if not already extant) a PersistentVolume tied to the
SharedVolume.FileSystemID and .AccessPointID.

In addition, the reconciliation loop will watch the [per-cluster resources](#per-cluster),
overwriting them wholesale if they change (they shouldn't).

## Finalization
Delete all the things.

The customer's PV may refuse to delete if the customer has not deleted the associated PVC and any pods using it.

## Limitations
The current version has the following limitations:
- One shared volume per cloud.
- Once the customer has bound a (namespaced) PersistentVolumeClaim to the PersistentVolume maintained by this operator:
    - All pods wishing to use the shared volume must be in that same namespace.
    - The namespace can't be changed.
    - If the PVC is deleted, the PV will be stuck and unusable without SRE intervention.
- The customer must manage the AWS side.

## Future
In future versions, we would like the operator to be able to:
- Handle multiple SharedVolumes.
    - Separate SharedVolumes need not be backed by separate EFS volumes, but by separate access points on the same EFS volume.
    - While the customer is managing the AWS side, it would be up to them whether to use separate EFS volumes or not, but
      once the operator is managing AWS resources (see below) we would want to use just one.
- Handle multiple namespaces associated with a single shared volume.
  The operator creates a separate PV for each (e.g. using the namespace as a suffix to the name to ensure uniqueness).
  At this point it would also make sense for the operator to take over creation/management of
  the associated PVCs.
    - We could simply namespace the CRD, thus requiring one SharedVolume per namespace.
      This makes it harder to tell which SharedVolumes point to the same underlying storage, and requires MxN resources.
    - Or we could keep the CRD namespaceless and add a `namespaces` list in the spec.
      This requires fewer resources and makes things easier to track, but seems less k8s-ish.
- Watch for and fix zombie PVs (bound to deleted PVCs)
- Manage the EFS volumes themselves.