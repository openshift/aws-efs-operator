# E2E Testing

## Good paths

* Single green path
    * Create SharedVolume
    * Validate PV and PVC created & bound
    * SharedVolume `oc get` should display the PVC name and `Ready` Phase.
    * Create pod against the PVC
    * Validate write access to the mount
    * Bounce pods, make sure access remains
    * Delete pod
    * Delete SharedVolume
    * Validate PV and PVC go away
    * Recreate and verify the data remains intact. ✔

* One SharedVolume, multiple pods
    * All pods must be in the same namespace (as the PVC, which is in the same namespace as the SharedVolume)
    * Distribute pods across multiple AZs and separate worker nodes within the same AZ
    * Verify shared write access (write from one pod is readable by another, etc.)

* Multiple SharedVolumes
    * Same FS/AP in different namespaces ✔
        * Validate shared write access from pods in each namespace ✔
    * Multiple APs, same FS (should appear as separate data stores) ✔
        * Separate pods ✔
        * Single pod with multiple volume defs ✔
    * Separate APs on separate FSes (should appear as separate data stores) ✔
        * Separate pods ✔
        * Single pod with multiple volume defs ✔

## Weird cases

* Identical SharedVolumes in the same namespace. (This should work, right? Though it's unnecessary.) ✔

## Operator resiliency

* Fadge "statics", e.g. edit/delete the DaemonSet, SCC, etc.
    * Verify they come back
    * Do this while one or more SharedVolumes exist
    * And while one or more pods are attached to their PVCs
    * Does FS access continue to work as long as the pod stays up?
    * Does the pod come back up if it is bounced?

* Delete SharedVolume while pod(s) still using managed PVC
    * Does the controller succeed in deleting the PV/PVC?
        * Yes, it thinks it does, but they stick around in `Terminating` state.
    * Does the SV get finalized and deleted successfully?
        * Yes
    * Does FS access continue to work as long as the pod stays up?
        * Yes
    * Does the pod come back up if it is bounced?
    * Does recreating the "same" SharedVolume make the pod resurrectable?

* Delete a managed PVC
    * **Known limitation.** Expect the operator to restore the PVC, but it and its PV won't rebind.
        * Can this be resolved by deleting the `SharedVolume` and recreating it?
            * If so, does the old PV get cleaned up or stay wedged?
    * Check same pod access questions (continued access until bounced, etc.)

* Delete a managed PV (Note that this shouldn't be possible IRL for customer)
    * Likewise

* Edit a SharedVolume
    * Expect the SV to be "unedited" and no effect on its PV/PVC

* Edit a managed PVC
    * It should be "restored" -- but unclear whether that will break the binding
        * If bits of the PVC we don't care about are edited (e.g. `capacity`?)
        * If bits that *do* matter are edited (like what if you change the `volumeName` (PV pointer)? The `storageClassName`?)

* Ditto PV

* Do above things while operator is down, bring operator back up, validate that things heal/work
    * Create SV, let PV/PVC create, create pod(s) against PVC
    * Operator down
    * Delete SV. This "hangs".
    * Bring up operator
    * Operator "deletes" PV and PVC.
        * This succeeds as far as the operator is concerned, but the PV/PVC stick around in `Terminating` state.
        * SV deletion completes.
        * Operator gets event and detects "out-of-band" SV deletion.
    * Destroy pod(s) associated with PVC associated with SV. This makes the PV/PVC disappear.