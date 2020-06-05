# Injected by boilerplate/openshift/golang_osd_cluster_operator/update
include boilerplate/openshift/golang_osd_cluster_operator/includes.mk

SHELL := /usr/bin/env bash

# TODO(efried): Clean up these temp files
COVER_PROFILE=coverage.out

# TODO(efried): Figure out how to force all go targets to
#    gvm use go1.13.6

default: gobuild

.PHONY: coverhtml
coverhtml: coverage
	go tool cover -html $(COVER_PROFILE)

.PHONY: update_boilerplate
update_boilerplate:
	@boilerplate/update

.PHONY: pr-check
pr-check:
	hack/app_sre_pr_check.sh
