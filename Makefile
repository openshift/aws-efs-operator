GOTEST_PACKAGES?=./...

test:
	go test $(GOTEST_PACKAGES)

cover:
	# TODO(efried): Clean up these temp files
	go test ${GOTEST_PACKAGES} -coverprofile /tmp/cp.out
	go tool cover -html /tmp/cp.out
