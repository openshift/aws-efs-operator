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

### Custom Resource Definition
This operator's Custom Resource shall be named **SharedVolume**.

Its spec shall contain:

| json            | go            | type   | required? | description |
|-|-|-|-|-|
| `fileSystemID`  | FileSystemID  | string | y         | The EFS volume identifier (e.g. `fs-484648c8`) |
| `accessPointID` | AccessPointID | string | y         | The access point identifier (e.g. `fsap-097bd0daaba932e64`) |
||||||

### AWS
In the current version, it is the customer's responsibility to create and maintain the necessary artifacts in AWS, per the
instructions in [this document](https://docs.google.com/document/d/1KdcqZirAdjZ2mJeqOKiMqiNTz4_VVZf7ePD5aB9RVXk).

This operator will accept the following AWS data as input:
- The EFS volume's file system ID.
- The ID of a file system access point (this allows the customer to control ownership/permissions of the NFS mount).

Note that only one EFS volume is required, since pods perceive each access point as a separate data store.
However, the operator will not prevent the use of multiple EFS volumes.

### Per Cluster (Initialization)
At the cluster level, the operator must create the following upon initialization:
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
A pod can only use a `PersistentVolumeClaim` in its namespace.
The `PersistentVolume` associated with the EFS CSI driver can only be bound to one `PersistentVolumeClaim`.
Thus, by extension, we must have one `PersistentVolume` per namespace, despite the fact that a
`PersistentVolume` is not inherently namespace-scoped.

Thus, for each `SharedVolume`, the operator will create and maintain:
- A `PersistentVolume` tied to the `SharedVolume.FileSystemID` and (optionally) `.AccessPointID`.
- A `PersistentVolumeClaim` against the PV, in the same namespace as the SharedVolume resource.

These artifacts will be owned by the operator.

### Reconciliation
On each iteration of the reconciliation loop, the operator shall react to:
- Changes to the cluster-level resources (which should really never happen):
  - Replace them wholesale.
- **New** `SharedVolume` resources:
  - Create the PV and PVC as described [above](#per-namespace).
- **Deleted** `SharedVolume` resources:
  - Delete the PVC and PV associated with the `SharedVolume`.
    This may fail until the customer has deleted any pods using the PVC, so the operator shouldn't wait for completion,
    but should continue to monitor the PV/PVC periodically or in the background until they are gone.
- **Changed** `SharedVolume` resources.
  It is not possible to edit a PersistentVolume, and rebinding a PVC is more trouble than it's worth.
  Thus when a `SharedVolume` is changed, we will simply:
  - Delete the PVC and PV as above.
  - Create a new PV and PVC as above.
- Changes to an operator-owned `PersistentVolume` or `PersistentVolumeClaim`.
  - It shouldn't actually be possible to edit a PV, or make changes to a PVC that actually matter,
    so the operator can (probably) ignore these.
  - If an operator-owned PV or PVC is deleted, the operator should ensure both of the related artifacts are
    deleted and should recreate both as if the associated `SharedVolume` were new.

### Finalization
Delete all the things.

The customer's PV and PVC may refuse to delete if the customer has not deleted any pods using the claim,
so the finalizers should continue to run until these artifacts are gone.

## Future
In future versions, we would like the operator to be able to manage the EFS volumes and access points.