//go:build !zetest

// Design: docs/architecture/container/image-store.md -- the store the engine entry builds
// Related: storeseam.go -- the factory the image store package installs
// Related: storeseam_zetest.go -- the same choice in the functional-test build

package container

// engineStoreFactory answers the factory the engine entry builds its store
// with: the one the image store package installed, or nil in a build that
// links none, where the supervisor refuses every start with the reason.
func engineStoreFactory() StoreFactory {
	return storeFactory
}
