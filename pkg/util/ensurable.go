package util

//go:generate mockgen -destination ../fixtures/zz_generated_mock_ensurable.go -package fixtures -source=ensurable.go Ensurable

/**
Types and functions to manage resources that can be "ensured" -- that is, created if they don't exist,
and updated to a canonical representation if they deviate from it.
*/

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	// "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// Ensurable provides helpers to allow ensuring the existence and state of a resource.
type Ensurable interface {
	// GetType returns a unique, empty runtime.Object of the specific type of the ensurable resource.
	GetType() crclient.Object
	// GetNamespacedName returns the `NamespacedName` for the resource. This can be used to identify
	// the Ensurable associated with a `reconcile.Request`.
	GetNamespacedName() types.NamespacedName
	// SetOwner sets the OwnerReferences field to a list of one element, the argument.
	SetOwner(*metav1.OwnerReference)
	// Ensure creates an Ensurable resource if it doesn't already exist, or updates it if it exists
	// and differs from the gold standard.
	Ensure(logr.Logger, crclient.Client) error
	// Delete makes sure the resource represented by the Ensurable is gone.
	Delete(logr.Logger, crclient.Client) error
}

// EnsurableImpl provides the implementation of the Ensurable interface.
type EnsurableImpl struct {
	ObjType        crclient.Object
	NamespacedName types.NamespacedName
	Definition     crclient.Object
	EqualFunc      func(local, server crclient.Object) bool
	owner          *metav1.OwnerReference
	latestVersion  crclient.Object
}

// GetType implements Ensurable.
func (e *EnsurableImpl) GetType() crclient.Object {
	// To make this "safe", we return a _copy_ of e.objType. The caller is expecting to be able to
	// use this e.g. to receive a real object from the server, and we don't want that data going into our
	// EnsurableImpl instance. For one thing, maybe the _next_ caller is expecting it to be empty. For another,
	// multiple threads using the same instance would be bad. Like crossing the streams.
	return e.ObjType.DeepCopyObject().(crclient.Object)
}

// GetNamespacedName implements Ensurable.
func (e *EnsurableImpl) GetNamespacedName() types.NamespacedName {
	return e.NamespacedName
}

// SetOwner implements Ensurable.
func (e *EnsurableImpl) SetOwner(owner *metav1.OwnerReference) {
	e.owner = owner
}

// Ensure implements Ensurable.
func (e *EnsurableImpl) Ensure(log logr.Logger, client crclient.Client) error {
	rname := e.GetNamespacedName()
	foundObj := e.GetType()
	if err := client.Get(context.TODO(), rname, foundObj); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Creating.", "resource", rname)
			_, newObj := e.latestDefinition(nil)
			// Clear any cached ResourceVersion, as required by Create
			newObj.(metav1.Object).SetResourceVersion("")
			if err = client.Create(context.TODO(), newObj); err != nil {
				log.Error(err, "Failed to create", "resource", rname)
				return err
			}
			log.Info("Created.", "resource", rname)
			// Cache it
			e.latestVersion = newObj
			return nil
		}
		log.Error(err, "Failed to retrieve.", "resource", rname)
		return err
	}
	log.Info("Found. Checking whether update is needed.", "resource", rname)

	equal, latestObj := e.latestDefinition(foundObj)
	if equal {
		log.Info("No update needed.")
	} else {
		log.Info("Update needed. Updating...")
		// Debug: print out _how_ the objects differ.
		// This will show what we're changing *from* as '-' and what we're changing *to* as '+'.
		log.V(2).Info(cmp.Diff(foundObj, latestObj))
		// Update uses ResourceVersion as a consistency marker to make sure an out-of-band update
		// didn't happen since our Get.
		latestObj.(metav1.Object).SetResourceVersion(foundObj.(metav1.Object).GetResourceVersion())
		if err := client.Update(context.TODO(), latestObj); err != nil {
			log.Error(err, "Failed to update.", "resource", rname)
			return err
		}
		log.Info("Updated.", "resource", rname)
	}
	// Okay, we either updated successfully or didn't need an update. The cache might be good, except:
	// - If this is the first hit for this resource, newObj is our generated skeleton definition, which
	//   is harder to compare, so we want to replace it with the version from the server.
	// - There's a chance newObj and foundObj differed, but in a way we didn't care about. Like maybe
	//   something changed and then changed back real quick so the only difference is the version
	//   marker.
	// So push the latest version into the cache.
	e.latestVersion = latestObj

	return nil
}

// Delete implements Ensurable
func (e *EnsurableImpl) Delete(log logr.Logger, client crclient.Client) error {
	// Let's clear the cache in case the object needs to be recreated at some point
	e.latestVersion = nil

	rname := e.GetNamespacedName()
	foundObj := e.GetType()
	if err := client.Get(context.TODO(), rname, foundObj); err != nil {
		if errors.IsNotFound(err) {
			// Already gone. Nothing to do
			return nil
		}
		// Some other error. That's bad
		log.Error(err, "Failed to retrieve.", "resource", rname)
		return err
	}

	log.Info("Deleting.", "resource", rname)
	if err := client.Delete(context.TODO(), foundObj); err != nil {
		if errors.IsNotFound(err) {
			// It got deleted out-of-band. That's fine
			return nil
		}
		log.Error(err, "Failed to delete.", "resource", rname)
		return err
	}

	// Cool.
	return nil
}

// AlwaysEqual is a convenience implementation of Ensurable.equalFunc for objects that can't change
// (in any significant way)
func AlwaysEqual(local, server crclient.Object) bool {
	return true
}

// EqualOtherThanMeta is a DeepEquals that ignores ObjectMeta and TypeMeta.
// Use when a DeepEqual on Spec won't work, e.g. when the meat of the object is at the top level
// and/or there _is_ no Spec.
func EqualOtherThanMeta(local, server crclient.Object) bool {
	return cmp.Equal(local, server, cmpopts.IgnoreTypes(metav1.ObjectMeta{}, metav1.TypeMeta{}))
}

func (e *EnsurableImpl) latestDefinition(serverObj crclient.Object) (bool, crclient.Object) {
	// If we cached one, use it, because it's not only right, it's complete
	def := e.latestVersion
	if def == nil {
		// Let this panic if none defined (developer error)
		def = e.Definition
		// Make sure this object triggers our watcher
		MakeMeCare(def)
	}
	// Make sure the owner reference is set, if applicable.
	// TODO(efried): It's a bit convoluted to have to do this here rather than within the above
	// condition. It's tightly coupled with the `equal` path that checks for the reference, and
	// the fact that we have a chicken/egg problem with statics being created on startup, before
	// we can count on the CRD existing.
	if e.owner != nil {
		def.(metav1.Object).SetOwnerReferences([]metav1.OwnerReference{*e.owner})
	}
	if serverObj != nil && e.equal(def, serverObj) {
		// If they're "equal", return the foundObj, because there are cases where it's newer or more complete
		return true, serverObj
	}

	// We were given:
	// - nothing to compare against; or
	// - a serverObj that doesn't "match".
	// We want the caller to use our cached/generated version.
	return false, def
}

// equal compares two objects, which are really of the type returned by `GetType`, and decides
// whether they are equal in all ways that matter to the controller. That doesn't necessarily
// mean they're 100% equal in all fields and values -- just the ones we care about.
// Importantly, the `local` object is either the `Definition` or pulled from the
// cache; and the `server` object is the one just retrieved from the server.
func (e *EnsurableImpl) equal(local, server crclient.Object) bool {
	// The server object must have its owner reference set, if applicable
	// NOTE(efried): Assumes we're the only one mucking with owner references
	if e.owner != nil && len(server.(metav1.Object).GetOwnerReferences()) != 1 {
		return false
	}
	// If these objects are versioned, and the versions are equal, there's no need to check further.
	if VersionsEqual(local, server) {
		return true
	}
	// All of the objects we care about need to not have their we-care-about-them markers changed.
	if !DoICare(server) {
		return false
	}
	// Run this specific ensurable's check
	return e.EqualFunc(local, server)
}

// VersionsEqual compares the generation of two objects. This can be used first in an `equal`
// because if the generation hasn't changed, there's no need to check further.
func VersionsEqual(local, server crclient.Object) bool {
	// If this isn't an object that tracks generation, this check isn't useful
	serverVersion := server.(metav1.Object).GetResourceVersion()
	if serverVersion == "" {
		return false
	}
	// Note that if `local` is generated (rather than cached) it will have an empty generation, which
	// is guaranteed to make this return `false`. That's fine.
	return serverVersion == local.(metav1.Object).GetResourceVersion()
}
