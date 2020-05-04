package test

import (
	"io/ioutil"
	"path/filepath"
	"testing"

	"sigs.k8s.io/yaml"
)

// LoadYAML loads a file called `filename` from the `testdata` folder relative to the
// package of the caller and unmarshals the contents of that file into `obj`.
// Any errors will fail the `t`est.
func LoadYAML(t *testing.T, obj interface{}, filename string) {
	bytes, err := ioutil.ReadFile(filepath.Join("testdata", filename))
	if err != nil {
		t.Fatal(err)
	}
	if err := yaml.Unmarshal(bytes, obj); err != nil {
		t.Fatal(err)
	}
}
