package controller

import (
	"2uasimojo/efs-csi-operator/pkg/controller/statics"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, statics.Add)
}
