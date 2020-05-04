package test

import (
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// diffOpts are common options we're passing into cmp.Diff.
var diffOpts cmp.Options

func init() {
	diffOpts = cmp.Options{
		// We want to ignore TypeMeta in all cases, because it's a given of the type itself.
		cmpopts.IgnoreTypes(metav1.TypeMeta{}),
		// We ignore the ResourceVersion because it gets set by the server and is unpredictable/opaque.
		// We ignore labels *in cmp.Diff* because sometimes we're checking a virgin resource definition
		// from a getter (label validation is done separately).
		cmpopts.IgnoreFields(metav1.ObjectMeta{}, "ResourceVersion", "Labels"),
	}
}
