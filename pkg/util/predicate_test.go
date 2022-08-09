package util

import (
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
	// metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

type testObjKind int

const (
	useNil testObjKind = iota
	care
	dontCare
)

// mkTestObj returns a new runtime.Object that's nil, or that our predicate methods should either
// care about or not, per the value of `kind`.
func mkTestObj(kind testObjKind) crclient.Object {
	if kind == useNil {
		return nil
	}
	o := &corev1.Pod{}
	if kind == care {
		MakeMeCare(o)
	}
	return o
}

// mkTestObj returns a new metav1.Object that's nil, or that our predicate methods should either
// care about or not, per the value of `kind`.
// func mkTestMeta(kind testObjKind) metav1.Object {
// 	obj := mkTestObj(kind)
// 	if obj == nil {
// 		return nil
// 	}
// 	return obj.(metav1.Object)
// }

// TestCreateDeleteGeneric covers
// - ICarePredicate
//   - .Create
//   - .Delete
//   - .Generic
// - DoICare
// - MakeMeCare
// - passes
func TestCreateDeleteGeneric(t *testing.T) {
	type test struct {
		name         string
		createEvent  event.CreateEvent
		deleteEvent  event.DeleteEvent
		genericEvent event.GenericEvent
		want         bool
	}
	mktest := func(name string, objKind, metaKind testObjKind, want bool) test {
		obj := mkTestObj(objKind)
		// meta := mkTestMeta(metaKind)
		return test{
			name,
			event.CreateEvent{
				Object: obj,
				// Meta:   meta,
			},
			event.DeleteEvent{
				Object: obj,
				// Meta:   meta,
			},
			event.GenericEvent{
				Object: obj,
				// Meta:   meta,
			},
			want,
		}
	}
	tests := []test{
		mktest("Care", care, care, true),
		// This is a case that shouldn't happen, because it means the meta isn't from the same object.
		// But it highlights that we pay attention to the object, not the meta.
		mktest("Care but meta is bogus", care, dontCare, true),
		// Ditto
		mktest("Don't care when meta cares but object is bogus", dontCare, care, false),
		mktest("Don't care, complete", dontCare, dontCare, false),
		mktest("Don't care because no object", useNil, care, false),
		// Even though the object cares, we consider lack of meta a don't-care
		mktest("Don't care no obj no meta", useNil, useNil, false),
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for name, got := range map[string]bool{
				"Create":  ICarePredicate.Create(tt.createEvent),
				"Delete":  ICarePredicate.Delete(tt.deleteEvent),
				"Generic": ICarePredicate.Generic(tt.genericEvent),
			} {
				if got != tt.want {
					t.Errorf("ICarePredicate.%s() = %v, want %v", name, got, tt.want)
				}
			}
		})
	}
}

// TestUpdate covers ICarePredicate.Update. (There's some overlap with the other pieces as well.)
func TestUpdate(t *testing.T) {
	type test struct {
		name        string
		updateEvent event.UpdateEvent
		want        bool
	}
	mktest := func(oldObj, newObj testObjKind, want bool) test {
		// Let's just generate the names for these
		name := fmt.Sprintf("oldObj(%v) newObj(%v)", oldObj, newObj)
		return test{
			name,
			event.UpdateEvent{
				ObjectOld: mkTestObj(oldObj),
				ObjectNew: mkTestObj(newObj),
				/*removed in newer version*/
				// MetaOld:   mkTestMeta(oldMeta),
				// MetaNew:   mkTestMeta(newMeta),
			},
			want,
		}
	}
	tests := []test{
		// We care when *either* the old or new passes
		mktest(care, care, true),
		mktest(care, dontCare, true),
		mktest(dontCare, care, true),
		mktest(care, useNil, true),
		mktest(useNil, care, true),
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ICarePredicate.Update(tt.updateEvent); got != tt.want {
				t.Errorf("ICarePredicate.Update() = %v, want %v", got, tt.want)
			}
		})
	}
}
