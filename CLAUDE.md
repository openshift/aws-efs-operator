# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

The AWS EFS Operator is a Kubernetes operator for OpenShift Dedicated that manages read-write-many access to AWS EFS volumes. It watches for `SharedVolume` custom resources and creates corresponding `PersistentVolume`/`PersistentVolumeClaim` pairs to enable EFS access point mounting across multiple pods, nodes, and availability zones.

## Common Development Commands

### Building and Testing
- `make test` - Run unit tests using envtest
- `make go-test` - Run Go tests directly with kubebuilder assets
- `make go-build` - Build the operator binary
- `make go-check` - Run static analysis and linting
- `make lint` - Run comprehensive linting including YAML validation
- `make coverage` - Generate code coverage reports

### Development Workflow
- `make default` - Run go-check, go-test, and go-build (the default make target)
- `make generate` - Generate code including CRDs and deep copy functions
- `make clean` - Clean build artifacts

### Container Operations
- `make docker-build` - Build operator container image
- `make docker-push` - Push container image to registry
- Uses podman by default, falls back to docker

## Architecture

### Core Components

**Custom Resource Definition**: `SharedVolume` (API version `aws-efs.managed.openshift.io/v1alpha1`)
- Spec contains `fileSystemID` (AWS EFS filesystem ID) and `accessPointID` (AWS EFS access point ID)
- Status tracks `phase` (Pending/Ready/Deleting/Failed), `claimRef` (PVC reference), and `message`

**Controllers**:
1. **Statics Controller** (`pkg/controller/statics/`) - Manages cluster-level resources needed for EFS CSI driver (CSIDriver, DaemonSet, ServiceAccount, SecurityContextConstraints, StorageClass)
2. **SharedVolume Controller** (`pkg/controller/sharedvolume/`) - Manages SharedVolume resources and their associated PV/PVC pairs

### Key Design Principles

- One PersistentVolume per namespace per SharedVolume (PVs can only bind to one PVC)
- SharedVolumes are namespace-scoped but can reference the same EFS access point across namespaces
- Generated PV/PVC names follow predictable patterns for resource management
- Immutable SharedVolume specs - editing attempts are reverted to prevent binding issues
- Finalizers ensure proper cleanup order (pods → PVCs → PVs → SharedVolumes)

### Package Structure

- `pkg/apis/awsefs/v1alpha1/` - SharedVolume CRD types and validation
- `pkg/controller/sharedvolume/` - Main SharedVolume reconciliation logic
- `pkg/controller/statics/` - EFS CSI driver infrastructure management
- `pkg/util/` - Shared utilities for resource manipulation and testing
- `pkg/fixtures/` - Test fixtures and sample resource definitions

### Build System

Uses OpenShift boilerplate conventions (`boilerplate/openshift/golang-osd-operator/`) which provides:
- Standardized Makefile targets for Go projects
- Container build/push workflows with podman/docker support
- FIPS compliance builds (enabled by default)
- Automated code generation and validation

The operator is built with Go 1.16 and uses controller-runtime v0.6.5 with Kubernetes 1.19.x APIs.