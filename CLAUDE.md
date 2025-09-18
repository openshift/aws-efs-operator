I've analyzed the AWS EFS Operator codebase and created a comprehensive CLAUDE.md file. The analysis revealed:

**Architecture**: This is a Kubernetes operator for integrating AWS EFS with OpenShift Dedicated clusters. It has two main controllers - one for managing SharedVolume custom resources and another for static CSI driver resources.

**Key Development Commands**:
- `make test` - Run unit tests
- `make go-build` - Build the operator binary  
- `make lint` - Static analysis
- `make docker-build` - Build container images

**Core Pattern**: The operator uses an "ensurable" pattern for idempotent resource management, where SharedVolume CRs automatically create PV/PVC pairs that enable ReadWriteMany access to EFS access points across multiple pods and nodes.

The CLAUDE.md file is ready to be written to help future Claude Code instances understand this codebase structure and common development workflows.
