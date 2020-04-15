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

The EFS volume's file system ID is required as input to the operator.

### Per Cluster
At the cluster level, the operator must create and maintain:
- A `CSIDriver` resource.
- A `DaemonSet` running the driver image.
    - This is restricted to running on worker nodes.
- A `StorageClass`.

We only need one instance of these artifacts per cluster, and their configuration will not change.
(It will in fact be identical from cluster to cluster, so the implementation may wish to maintain these
resource definitions in a textual format such as YAML, rather than modeled in go.)

### Per EFS Volume
NFS (the protocol underlying EFS) initializes the mount with root user and group ownership (`uid=0, gid=0`) and `755` permissions.
Customer pods normally do not run as root, nor do we want that to be a requirement (see [Objectives](#objectives)).
This operator will therefore be responsible for ensuring that the ownership and/or permissions of the mount are set as
directed by the [configuration](#custom-resource-definition). This will be effected by creating:
- A Namespace owned by the operator;
- A PersistentVolumeClaim in that Namespace, bound to
- A PersistentVolume tied to the customer-provided EFS volume ID ([why we need this](#per-namespace));
- A Pod in the Namespace with:
    - The EFS volume mounted
    - A process watching the NFS mount inode and reconciling its ownership/permissions as specified in the CR.

The current version only supports one shared volume per cluster.

### Per Namespace
A pod can only use a PersistentVolumeClaim in its namespace.
The PersistentVolume associated with the EFS CSI driver can only be bound to one PersistentVolumeClaim.
Thus, by extension, we must have one PersistentVolume per namespace, despite the fact that a
PersistentVolume is not inherently namespace-scoped.

The current version only supports a single PV/PVC/namespace (on the customer side -- the operator still
[needs a PV/PVC in its namespace](#per-efs-volume) to monitor the NFS permissions).
So at the moment this reduces to the operator simply creating and managing a single PV.

## Custom Resource Definition
This operator's Custom Resource shall be named **SharedVolume**.

Its namespace shall be `efs-csi-operator`.

Its spec shall contain:

| json          | go          | type   | required? | description |
|-|-|-|-|-|
| `efsVolumeID` | EFSVolumeID | string | y         | The EFS volume identifier (e.g. `fs-484648c8`) |
| `nfsMount`    | NFSMount    | map    | n         | Configuration for the NFS mount |
||||||

The `nfsMount` map contains:

| json          | go          | type   | required? | description |
|-|-|-|-|-|
| `ownerUID`    | OwnerUID    | int    | no        | Numeric id of the desired owning user of the mount  |
| `ownerGID`    | OwnerGID    | int    | no        | Numeric id of the desired owning group of the mount |
| `mode`        | Mode        | string | no        | Numeric permissions mode, as a string, e.g. `04775` |
||||||

Because the current version only has support for a single EFS volume, the operator shall refuse to
create more than one SharedVolume CR.

Due to the complications of managing PVC bindings, the current version will also refuse to allow the
`efsVolumeID` to be edited once set.
In order to switch to another file system, the SharedVolume must be deleted and a new one created.

## Initialization
On startup, the operator will create:

| kind | namespace | description |
|-|-|-|
| DaemonSet | `kube-system` | Runs the CSI driver image |
| CSIDriver | - ||
| StorageClass | - ||
| Namespace | `efs-csi-operator` | Namespace for operator-owned resources and the customer's CR |
||||

## Reconciliation
The reconciliation loop will ensure (create if not already extant) the following:

- Operator-owned resources for NFS mount management:
  - PersistentVolume tied to the SharedVolume.EFSVolumeID
  - PersistentVolumeClaim in the `efs-csi-operator` namespace, bound to the above PV.
  - Pod with
    - the EFS volume mounted;
    - a process monitoring the NFS mount inode.
        - If SharedVolume.NFSMount.OwnerUID is set and differs from the inode's owning user, `chown` the inode.
        - If SharedVolume.NFSMount.OwnerGID is set and differs from the inode's owning group, `chown` the inode.
        - If SharedVolume.NFSMount.Mode is set and differs from the inode's permission mode, `chmod` the inode.
  - As an optimization, if SharedVolume.NFSMount is absent or empty (the customer is declaring that they don't
    need the mount managed -- perhaps because they happen to have root pod capabilities in their cluster and can
    do it themselves) this PV, PVC, and Pod are not necessary and can be skipped.
- PersistentVolume (for the customer's use) tied to the SharedVolume.EFSVolumeID.

In addition, the reconciliation loop will watch the DaemonSet, CSIDriver, and StorageClass resources,
overwriting them wholesale if they change (they shouldn't).

## Finalization
Delete all the things.

The customer's PV may refuse to delete if the customer has not deleted the associated PVC.

## Limitations
The current version has the following limitations:
- One shared volume per cloud.
- Once the customer has bound a (namespaced) PersistentVolumeClaim to the PersistentVolume maintained by this operator:
    - All pods wishing to use the shared volume must be in that same namespace.
    - The namespace can't be changed.
    - If the PVC is deleted, the PV will be stuck and unusable without SRE intervention.
- The customer must manage the AWS side.
- The DaemonSet must run in the `kube-system` namespace.

## Future
In future versions, we would like the operator to be able to:
- Handle multiple EFS volumes.
    - To manage NFS permissions, we will need one operator-owned PV/PVC pair per volume.
      However, it should be possible to mount and monitor all of the volumes in a single operator-owned pod
      rather than using a separate pod for each.
- Handle multiple namespaces associated with a single EFS volume.
    - This could be managed by including a `namespaces` list in the CRD spec.
      The operator creates a separate PV for each (using the namespace as a suffix to the name).
      At this point it would also make sense for the operator to take over creation/management of
      the associated PVCs.
- Watch for and fix zombie PVs (bound to deleted PVCs)
- Manage the EFS volumes themselves.