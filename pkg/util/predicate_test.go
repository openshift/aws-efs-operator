package util

import (
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
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
func mkTestObj(kind testObjKind) runtime.Object {
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
func mkTestMeta(kind testObjKind) metav1.Object {
	obj := mkTestObj(kind)
	if obj == nil {
		return nil
	}
	return obj.(metav1.Object)
}

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
		meta := mkTestMeta(metaKind)
		return test{
			name,
			event.CreateEvent{
				Object: obj,
				Meta:   meta,
			},
			event.DeleteEvent{
				Object: obj,
				Meta:   meta,
			},
			event.GenericEvent{
				Object: obj,
				Meta:   meta,
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
		mktest("Don't care because no meta", care, useNil, false),
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
	mktest := func(oldObj, oldMeta, newObj, newMeta testObjKind, want bool) test {
		// Let's just generate the names for these
		name := fmt.Sprintf("oldObj(%v) oldMeta(%v) newObj(%v) newMeta(%v)", oldObj, oldMeta, newObj, newMeta)
		return test{
			name,
			event.UpdateEvent{
				ObjectOld: mkTestObj(oldObj),
				ObjectNew: mkTestObj(newObj),
				MetaOld:   mkTestMeta(oldMeta),
				MetaNew:   mkTestMeta(newMeta),
			},
			want,
		}
	}
	tests := []test{
		// We care when *either* the old or new passes
		mktest(care, care, care, care, true),
		mktest(care, care, care, dontCare, true),
		mktest(care, care, care, useNil, true),
		mktest(care, care, dontCare, care, true),
		mktest(care, care, dontCare, dontCare, true),
		mktest(care, care, dontCare, useNil, true),
		mktest(care, care, useNil, care, true),
		mktest(care, care, useNil, dontCare, true),
		mktest(care, care, useNil, useNil, true),
		// For these, old still passes because we pay attention to the obj, not the meta.
		mktest(care, dontCare, care, care, true),
		mktest(care, dontCare, care, dontCare, true),
		mktest(care, dontCare, care, useNil, true),
		mktest(care, dontCare, dontCare, care, true),
		mktest(care, dontCare, dontCare, dontCare, true),
		mktest(care, dontCare, dontCare, useNil, true),
		mktest(care, dontCare, useNil, care, true),
		mktest(care, dontCare, useNil, dontCare, true),
		mktest(care, dontCare, useNil, useNil, true),
		// The old side is bogus, translating to don't-care; so we only care if the new side passes
		mktest(care, useNil, care, care, true),
		mktest(care, useNil, care, dontCare, true),
		mktest(care, useNil, care, useNil, false),
		mktest(care, useNil, dontCare, care, false),
		mktest(care, useNil, dontCare, dontCare, false),
		mktest(care, useNil, dontCare, useNil, false),
		mktest(care, useNil, useNil, care, false),
		mktest(care, useNil, useNil, dontCare, false),
		mktest(care, useNil, useNil, useNil, false),
		mktest(dontCare, care, care, care, true),
		mktest(dontCare, care, care, dontCare, true),
		mktest(dontCare, care, care, useNil, false),
		mktest(dontCare, care, dontCare, care, false),
		mktest(dontCare, care, dontCare, dontCare, false),
		mktest(dontCare, care, dontCare, useNil, false),
		mktest(dontCare, care, useNil, care, false),
		mktest(dontCare, care, useNil, dontCare, false),
		mktest(dontCare, care, useNil, useNil, false),
		mktest(dontCare, dontCare, care, care, true),
		mktest(dontCare, dontCare, care, dontCare, true),
		mktest(dontCare, dontCare, care, useNil, false),
		mktest(dontCare, dontCare, dontCare, care, false),
		mktest(dontCare, dontCare, dontCare, dontCare, false),
		mktest(dontCare, dontCare, dontCare, useNil, false),
		mktest(dontCare, dontCare, useNil, care, false),
		mktest(dontCare, dontCare, useNil, dontCare, false),
		mktest(dontCare, dontCare, useNil, useNil, false),
		mktest(dontCare, useNil, care, care, true),
		mktest(dontCare, useNil, care, dontCare, true),
		mktest(dontCare, useNil, care, useNil, false),
		mktest(dontCare, useNil, dontCare, care, false),
		mktest(dontCare, useNil, dontCare, dontCare, false),
		mktest(dontCare, useNil, dontCare, useNil, false),
		mktest(dontCare, useNil, useNil, care, false),
		mktest(dontCare, useNil, useNil, dontCare, false),
		mktest(dontCare, useNil, useNil, useNil, false),
		mktest(useNil, care, care, care, true),
		mktest(useNil, care, care, dontCare, true),
		mktest(useNil, care, care, useNil, false),
		mktest(useNil, care, dontCare, care, false),
		mktest(useNil, care, dontCare, dontCare, false),
		mktest(useNil, care, dontCare, useNil, false),
		mktest(useNil, care, useNil, care, false),
		mktest(useNil, care, useNil, dontCare, false),
		mktest(useNil, care, useNil, useNil, false),
		mktest(useNil, dontCare, care, care, true),
		mktest(useNil, dontCare, care, dontCare, true),
		mktest(useNil, dontCare, care, useNil, false),
		mktest(useNil, dontCare, dontCare, care, false),
		mktest(useNil, dontCare, dontCare, dontCare, false),
		mktest(useNil, dontCare, dontCare, useNil, false),
		mktest(useNil, dontCare, useNil, care, false),
		mktest(useNil, dontCare, useNil, dontCare, false),
		mktest(useNil, dontCare, useNil, useNil, false),
		mktest(useNil, useNil, care, care, true),
		mktest(useNil, useNil, care, dontCare, true),
		mktest(useNil, useNil, care, useNil, false),
		mktest(useNil, useNil, dontCare, care, false),
		mktest(useNil, useNil, dontCare, dontCare, false),
		mktest(useNil, useNil, dontCare, useNil, false),
		mktest(useNil, useNil, useNil, care, false),
		mktest(useNil, useNil, useNil, dontCare, false),
		mktest(useNil, useNil, useNil, useNil, false),
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ICarePredicate.Update(tt.updateEvent); got != tt.want {
				t.Errorf("ICarePredicate.Update() = %v, want %v", got, tt.want)
			}
		})
	}
}
