# EFS CSI Operator for OpenShift Dedicated

Yar.

This be a `go` operator to manage read-write-many access to AWS EFS volumes in an OpenShift Dedicated cluster.
It is a work in progress. Stay tuned.

In the meantime, you may want to check out the [design](DESIGN.md).

## Go
This project is developed against go version 1.13.6.
To avoid surprises, you should use the same version when developing and testing locally.
One handy way to do that is via [gvm](https://github.com/moovweb/gvm).

    gvm use go1.13.6

## Operator SDK
This project was bootstrapped using [v0.16.0 of operator-sdk](https://github.com/operator-framework/operator-sdk/releases/tag/v0.16.0).
Please use that version for further code generation and similar activities.