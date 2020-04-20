package controller

import (
	"2uasimojo/efs-csi-operator/pkg/controller/sharedvolume"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, sharedvolume.Add)
}
