SHELL := /usr/bin/env bash

# TODO(efried): Clean up these temp files
COVER_PROFILE=coverage.out

# TODO(efried): Figure out how to force all go targets to
#    gvm use go1.13.6

# Include shared Makefiles
include project.mk
include standard.mk

default: gobuild

.PHONY: coverhtml
coverhtml: coverage
	go tool cover -html $(COVER_PROFILE)

# Build the docker image
.PHONY: docker-build
docker-build: build

# Push the docker image
.PHONY: docker-push
docker-push: push

.PHONY: pr-check
pr-check:
	hack/app_sre_pr_check.sh
