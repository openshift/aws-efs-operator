# Design

## Objectives
- Access to a **shared writable file system**
- from **multiple pods**
- even when those pods are on **different worker nodes**, and
- even when those nodes are in **different availability zones**.
- Do not require the customer to possess any special permissions, roles, etc.

## Environment
This operator supports:
- **OpenShift Dedicated** with
- **CCS clusters** backed by
- **AWS**.
- **Shared storage** backed by
- **EFS volumes**, by way of
- The **CSI driver**.

## Infrastructure

### Go
This project is developed against go version 1.13.6.
To avoid surprises, you should use the same version when developing and testing locally.
One handy way to do that is via [gvm](https://github.com/moovweb/gvm).

    gvm use go1.13.6

### Operator SDK
This project was bootstrapped using [v0.16.0 of operator-sdk](https://github.com/operator-framework/operator-sdk/releases/tag/v0.16.0).
Please use that version for further code generation and similar activities.

## Artifacts

### Custom Resource Definition
This operator's Custom Resource shall be named **SharedVolume**.

Its Spec shall contain:

| json            | go            | type   | required? | description |
| -               | -             | -      | -         | -           |
| `fileSystemID`  | FileSystemID  | string | y         | The EFS volume identifier (e.g. `fs-1234cdef`) |
| `accessPointID` | AccessPointID | string | y         | The access point identifier (e.g. `fsap-0123456789abcdef`) |
|                 |               |        |           |             |

Its Status shall contain:
| json       | go       | type                      | description |
| -          | -        | -                         | -           |
| `claimRef` | ClaimRef | TypedLocalObjectReference | Reference to the PVC created at the behest of this `SharedVolume`. This is the (only) thing the consumer needs to know to build the spec of a pod using the volume. |
| `phase`    | Phase    | string                    | String indicating the state of the PV/PVC associated with this SharedVolume. Possible values are "Pending", "Ready", "Deleting", "Failed". (The name "`Phase`" and the values are roughly inspired by what's seen in `PersistentVolumeStatus`) |
| `message`  | Message  | string                    | Human-readable information augmenting the `Phase`. (Will probably just be the latest error string when `phase` is `Failed`, and empty otherwise.) |
|            |          |                           |             |

### AWS
In the current version, it is the customer's responsibility to create and maintain the necessary artifacts in AWS, per the
instructions in [this document](https://access.redhat.com/articles/5025181).

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
- A `ServiceAccount` and `SecurityContextConstraints` giving the
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
- A `PersistentVolume` tied to the `SharedVolume.FileSystemID` and `.AccessPointID`.
- A `PersistentVolumeClaim` against the PV, in the same namespace as the `SharedVolume` resource.

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
    but should continue to attempt deletion until successful. Only then should the SharedVolume's finalizer be removed.
- **Changed** `SharedVolume` resources.
  It is not possible to edit a PersistentVolume, and rebinding a PVC is more trouble than it's worth.
  Thus when a `SharedVolume` is changed, we will simply un-edit it, restoring the original `Spec` values, which will be discovered from the associated PV.
- Changes to an operator-owned `PersistentVolume` or `PersistentVolumeClaim`.
  - It shouldn't actually be possible to edit a PV, or make changes to a PVC that actually matter,
    so the operator can (probably) ignore these. But overwrite with the golden definition anyway.
  - If an operator-owned PV or PVC is deleted, the operator should ensure both of the related artifacts are
    deleted and should recreate both as if the associated `SharedVolume` were new. (TODO: we're not doing this at the moment.)

## Future

* We would like the operator to be able to manage the EFS volumes and access points.
* Better resiliency of PV/PVC binding problems.
* Truly disallow editing a `SharedVolume`.