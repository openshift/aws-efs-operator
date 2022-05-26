FIPS_ENABLED=true
include boilerplate/generated-includes.mk

# TODO: Remove once app-iterface ci.ext job is gone
.PHONY: pr-check
pr-check:
	hack/app_sre_pr_check.sh

.PHONY: boilerplate-update
boilerplate-update:
	@boilerplate/update
