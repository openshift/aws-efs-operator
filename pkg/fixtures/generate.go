package fixtures

// This is a dummy source file whose job is to contain the directives to (re)produce the generated
// mock fixtures in this package.
// Run `make generate` from the project root.
// Dependency: mockgen, qua:
//    GO111MODULE=on go get github.com/golang/mock/mockgen@latest

//go:generate mockgen -destination ./mock_crclient.go -package fixtures sigs.k8s.io/controller-runtime/pkg/client Client
//go:generate mockgen -destination ./mock_logr.go -package fixtures github.com/go-logr/logr Logger
