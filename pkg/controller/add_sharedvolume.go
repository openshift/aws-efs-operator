package controller

import (
	"openshift/aws-efs-operator/pkg/controller/sharedvolume"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, sharedvolume.Add)
}
