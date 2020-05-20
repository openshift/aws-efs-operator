# TODO
Short-term things that need fixing/addressing

* Fix `oc explain`:

```shell
[efried@efried ops-sop]$ oc explain sv
KIND:     SharedVolume
VERSION:  aws-efs.managed.openshift.io/v1alpha1

DESCRIPTION:
     <empty>
```

* Set up permissions so the customer can create `SharedVolume` CRs.
    * On that note, is there an RBACism that will forbid them from editing a `SharedVolume` once it's created?
    That would make some things easier (like allowing us to remove `uneditSharedVolume`).
    For instance, I saw this when trying to edit a pod:
    ```
    # pods "pod4" was not valid:
    # * spec: Forbidden: pod updates may not change fields other than `spec.containers[*].image`, `spec.initContainers[*].image`, `spec.activeDeadlineSeconds` or `spec.tolerations` (only additions to existing tolerations)
    ```
    But that may be a core kubernetes-ism.
    There seems to be something called "Admission Webhook" that might work, but it's apparently very heavy.
    https://github.com/kubernetes/enhancements/blob/master/keps/sig-api-machinery/20190603-immutable-fields.md is on the way,
    but not landed (https://github.com/kubernetes/enhancements/issues/1101).
