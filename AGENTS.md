# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What This Operator Does

aws-efs-operator manages read-write-many access to AWS EFS volumes in OpenShift Dedicated (OSD) CCS clusters. Users create **SharedVolume** custom resources specifying an EFS file system ID and access point ID; the operator provisions a bound PersistentVolume/PersistentVolumeClaim pair so pods can mount the EFS storage.

- API Group: `aws-efs.managed.openshift.io/v1alpha1`
- CRD: `SharedVolume` (shortname: `sv`, namespaced)
- Operator namespace: `openshift-aws-efs`

## Build Commands

```bash
make                   # Default: go-check + go-test + go-build
make go-build          # Compile binary to build/_output/bin/aws-efs-operator
make go-check          # Lint (golangci-lint)
make go-test           # Unit tests (uses envtest/kubebuilder assets)
make generate          # Regenerate deepcopy, CRDs, mocks, go-bindata
make validate          # Boilerplate freeze check + generate-check
make coverage          # Code coverage report
```

Run a single package's tests:
```bash
TESTTARGETS=./pkg/controller/sharedvolume/... make go-test
```

Verbose test output:
```bash
TESTOPTS=-v make go-test
```

Run in CI-equivalent container:
```bash
./boilerplate/_lib/container-make go-test
```

## Architecture

### Two-Controller Design

1. **Statics Controller** (`pkg/controller/statics/`) — Ensures cluster-level infrastructure exists: ServiceAccount, SecurityContextConstraints, DaemonSet (EFS CSI driver on every worker), CSIDriver, and StorageClass. Resource definitions are embedded YAML in `pkg/controller/statics/defs/` via go-bindata. If any static resource drifts from its golden definition, the controller overwrites it.

2. **SharedVolume Controller** (`pkg/controller/sharedvolume/`) — Reconciles SharedVolume CRs:
   - **Create**: Provisions a PV + PVC pair, sets a finalizer, marks status `Ready`
   - **Edit**: Reverts spec changes (SharedVolume spec is immutable after creation)
   - **Delete**: Removes PV/PVC, then removes finalizer

### Cross-Namespace Ownership

PVs are cluster-scoped so OwnerReferences can't link them to namespaced SharedVolumes. Instead, custom labels track ownership:
- `openshift.io/aws-efs-operator-shared-volume-owner-namespace`
- `openshift.io/aws-efs-operator-shared-volume-owner-name`

The mapper (`pkg/controller/sharedvolume/mapper.go`) reads these labels to route PV/PVC events back to the owning SharedVolume's reconciler.

### Ensurable Pattern

`pkg/util/ensurable.go` defines an `Ensurable` interface for idempotent create-or-update-to-match-golden-definition. Both controllers use this: statics for the five cluster resources, SharedVolume controller for PV and PVC. Each ensurable has a custom `EqualFunc` to decide whether the live resource matches the desired state.

### Entrypoint

`cmd/manager/main.go` — Sets up the controller-runtime manager, bootstraps static resources via `statics.EnsureStatics()`, then starts both controllers.

## Code Generation

Several files are generated and must be committed:
- `pkg/apis/awsefs/v1alpha1/zz_generated.deepcopy.go` — deepcopy (controller-gen)
- `pkg/controller/statics/zz_generated_defs.go` — embedded YAML (go-bindata from `defs/`)
- `pkg/fixtures/zz_generated_mock_*.go` — mocks (mockgen)

After changing types, static YAML defs, or mock interfaces, run `make generate` and commit the results. CI runs `make generate-check` to verify generated files are up to date.

## Key Dependencies

- operator-sdk v0.18.2 (old-style layout: `pkg/apis`, `cmd/manager`)
- controller-runtime v0.6.5
- k8s.io libs pinned to v0.19.7 (via go.mod replace directives)
- Kubernetes envtest for unit tests (no real cluster needed)
- gomock for mock-based error-path testing

## Boilerplate

Build infrastructure comes from `boilerplate/openshift/golang-osd-operator/`. The repo Makefile is minimal — it sets `FIPS_ENABLED=true` and includes `boilerplate/generated-includes.mk` which pulls in standard targets. Update boilerplate with `make boilerplate-update`.

## Testing

Tests use two patterns:
- **Fake client tests** — controller-runtime's `fake.NewFakeClient` for happy-path reconciliation
- **Mock tests** — gomock-generated mocks (`pkg/fixtures/`) for error injection

Test helpers live in `pkg/test/` (LoadYAML, DoDiff, test client builders).

## Design Documents

- `DESIGN.md` — Architecture rationale, CRD spec, reconciliation behavior
- `TESTING.md` — Manual and automated test scenarios
- `TODO.md` — Known issues and future work
