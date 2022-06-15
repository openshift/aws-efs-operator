package test

import (
	util "openshift/aws-efs-operator/pkg/util"

	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"
)

// LoadYAML loads a file called `filename` from the `testdata` folder relative to the
// package of the caller and unmarshals the contents of that file into `obj`.
// Any errors will fail the `t`est.
func LoadYAML(t *testing.T, obj interface{}, filename string) {
	filename = filepath.Join("testdata", filename)
	bytes, err := ioutil.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	if err := yaml.Unmarshal(bytes, obj); err != nil {
		t.Fatal(err)
	}
}

// DoDiff deep-compares two `runtime.Object`s and fails the `t`est with a useful message (showing
// the diff) if they differ. `expectLabel` indicates whether we expect `actual` to be labeled for
// our watchers.
func DoDiff(t *testing.T, expected, actual runtime.Object, expectLabel bool) {
	diff := cmp.Diff(expected, actual, diffOpts...)
	if diff != "" {
		t.Fatal("Objects differ: -expected, +actual\n", diff)
	}
	if doICare := util.DoICare(actual); expectLabel != doICare {
		t.Fatalf("expectLabel was %v but DoICare returned %v", expectLabel, doICare)
	}
}
