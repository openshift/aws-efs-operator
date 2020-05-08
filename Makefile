GOTEST_PACKAGES?=./cmd/... ./pkg/...

generate:
	go generate $(GOTEST_PACKAGES)
	# Don't forget to commit generated files

test:
	go test $(GOTEST_PACKAGES)

cover:
	# TODO(efried): Clean up these temp files
	go test ${GOTEST_PACKAGES} -coverprofile /tmp/cp.out
	go tool cover -html /tmp/cp.out
