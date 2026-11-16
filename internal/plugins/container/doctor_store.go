// Design: docs/architecture/container/image-store.md -- the doctor codes the image store owns
// Related: doctor.go -- the runtime's codes and checks, registered beside these
// Related: register.go -- registerContainerDiagnosticCodes, which publishes both tables

package container

import (
	"errors"

	"github.com/ze-software/ze/internal/core/diagnostic"
)

// The doctor codes of the image store, one per problem class of its doctor
// table (plan/post-release/features/spec-container-2-image-store.md, "Doctor
// checks and codes"). The image-absent and store-orphan codes are the
// operator surface package's, so they are not here.
const (
	// The state of the store, reported by the runtime check container-store.
	codeStoreUnwritable = "doctor-container-store-unwritable"
	codeStoreFull       = "doctor-container-store-full"
	// The pair of the kernel capability container-quota, in the scheme of
	// every container kernel capability (doctor.go).
	codeKernelQuotaUnavailable = "doctor-container-kernel-quota-unavailable"
	codeKernelQuotaUnknown     = "doctor-container-kernel-quota-unknown"
	// A writable layer or a volume at its cap, reported by the runtime check
	// container-storage-usage.
	codeStorageFull = "doctor-container-storage-full"
	// The failures of one fetch, each raised on the report bus.
	codeRegistryUnreachable = "doctor-container-registry-unreachable"
	codeRegistryAuthRefused = "doctor-container-registry-auth-refused"
	codeImageNotFound       = "doctor-container-image-not-found"
	codeImageDigestMismatch = "doctor-container-image-digest-mismatch"
	codeImageRefused        = "doctor-container-image-refused"
	codeFetchNoSpace        = "doctor-container-fetch-no-space"
)

// The commands the explanations name.
const (
	exampleDeleteAllImages = "ze delete container image all"
	exampleFetchImage      = "ze create container image reference REFERENCE"
)

// storeDiagnosticCodes is the explanation of each code of the image store.
// It is registered beside containerDiagnosticCodes, from the plugin's own
// package, so a build without the plugin holds none of them.
var storeDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:        codeStoreUnwritable,
		Title:       "The container image store cannot be written",
		Description: "The store folder under the data directory is absent, cannot be written, or is open to group or others. Ze keeps every image, writable layer and volume there, so no image can be fetched and no container can start on a new image. A store open to others would let a local user reach a set-user file of an image. On a distribution, give the folder to root with mode 0700. On the appliance, the persistent partition did not mount read-write: read the kernel log with ze show system kernel-log.",
		Examples:    []string{exampleShowDoctor, "ze show container store", "ze show diagnostic " + codeStoreUnwritable},
		HostVerdict: true,
	},
	{
		Code:        codeStoreFull,
		Title:       "The container image store is below its free-space reserve",
		Description: "The filesystem of the data directory holds less free space than the store keeps in reserve, so a fetch is refused rather than fill the filesystem the configuration also lives on. Delete the images no container holds.",
		Examples:    []string{exampleShowDoctor, exampleDeleteAllImages, "ze show diagnostic " + codeStoreFull},
	},
	{
		Code:        codeKernelQuotaUnavailable,
		Title:       "The data directory cannot cap a container's storage",
		Description: "The configuration holds a container to run, and the filesystem of the data directory does not enforce ext4 project quota. Every writable layer and every volume has a size cap the kernel enforces, and Ze never replaces it with a measured cap, so the daemon refuses to start, a reload is refused and ze config validate fails. Point ze.data.dir at an ext4 filesystem mounted with project quota enforced. On the appliance the persistent partition is one, so the partition did not mount as built: read the kernel log with ze show system kernel-log.",
		Examples:    []string{exampleShowDoctor, "ze show diagnostic " + codeKernelQuotaUnavailable},
		HostVerdict: true,
	},
	{
		Code:        codeKernelQuotaUnknown,
		Title:       "Ze could not tell whether the data directory can cap a container's storage",
		Description: "The probe of the data directory's filesystem could not read whether ext4 project quota is enforced there. Nothing is refused on an unknown answer. The finding names what could not be read. Make the data directory readable to the daemon, or point ze.data.dir at an ext4 filesystem mounted with project quota enforced.",
		Examples:    []string{exampleShowDoctor, "ze show diagnostic " + codeKernelQuotaUnknown},
		HostVerdict: true,
	},
	{
		Code:        codeStorageFull,
		Title:       "A container's writable layer or volume is at its cap",
		Description: "A container wrote as much into its writable layer or into one of its volumes as the configured size allows, and further writes fail inside the container with disk quota exceeded. The finding names the container and the layer or the volume. Clear the writable layer, or raise the size in the container's configuration.",
		Examples:    []string{exampleShowDoctor, "ze clear container layer name NAME", "ze show diagnostic " + codeStorageFull},
	},
	{
		Code:        codeRegistryUnreachable,
		Title:       "A fetch could not reach the image registry",
		Description: "A fetch failed before the registry answered: the name did not resolve, the connection failed, TLS was refused, the registry answered a server error, or no byte arrived for 60 seconds. Checked blobs stay for the next attempt. Run the fetch again. For a registry signed by a private authority, set the registry's trust-anchor to its pki ca entry.",
		Examples:    []string{exampleFetchImage, "ze show diagnostic " + codeRegistryUnreachable},
	},
	{
		Code:        codeRegistryAuthRefused,
		Title:       "The image registry refused the credentials or the access",
		Description: "The registry answered that the credentials are wrong or that they give no access to the repository. Set the registry's credentials in the configuration.",
		Examples:    []string{"set container registry HOST authentication username NAME password PASSWORD", "ze show diagnostic " + codeRegistryAuthRefused},
	},
	{
		Code:        codeImageNotFound,
		Title:       "The image registry holds no such image or tag",
		Description: "The registry answered that the repository, the tag or the digest does not exist. Correct the container's image in the configuration, or fetch the right reference.",
		Examples:    []string{exampleFetchImage, "ze show diagnostic " + codeImageNotFound},
	},
	{
		Code:        codeImageDigestMismatch,
		Title:       "A fetched image did not match its digest or its size",
		Description: "A manifest, an image configuration or a layer did not hash to the digest it was asked for, or did not have its declared size, so Ze stored none of it. Run the fetch again. When a mirror served it, remove the mirror from the registry's configuration.",
		Examples:    []string{exampleFetchImage, "delete container registry HOST mirror", "ze show diagnostic " + codeImageDigestMismatch},
	},
	{
		Code:        codeImageRefused,
		Title:       "An image was refused",
		Description: "The image holds an entry Ze does not apply, passes a bound of the store, uses a type Ze does not accept, or offers no entry for this platform. The finding names the entry, the bound or the type. For a bound, raise it in the configuration. Otherwise name another image.",
		Examples:    []string{"set container store largest-image SIZE", "ze show diagnostic " + codeImageRefused},
	},
	{
		Code:        codeFetchNoSpace,
		Title:       "A fetch was refused for lack of space",
		Description: "The image needs more space than the data directory's filesystem holds above the store's reserve, so the fetch was refused before any blob was downloaded. Delete the images no container holds.",
		Examples:    []string{exampleDeleteAllImages, "ze show container store", "ze show diagnostic " + codeFetchNoSpace},
	},
}

// registerStoreEventCodes declares the codes a fetch raises on the report
// bus, so the recent-events check declares them. Each is a composite literal
// with a constant code, the form the doctor-coverage gate reads. It runs after
// storeDiagnosticCodes registered: an event code needs its explanation first.
func registerStoreEventCodes() error {
	return errors.Join(
		diagnostic.RegisterEventCode(diagnostic.EventCode{Code: codeRegistryUnreachable}),
		diagnostic.RegisterEventCode(diagnostic.EventCode{Code: codeRegistryAuthRefused}),
		diagnostic.RegisterEventCode(diagnostic.EventCode{Code: codeImageNotFound}),
		diagnostic.RegisterEventCode(diagnostic.EventCode{Code: codeImageDigestMismatch}),
		diagnostic.RegisterEventCode(diagnostic.EventCode{Code: codeImageRefused}),
		diagnostic.RegisterEventCode(diagnostic.EventCode{Code: codeFetchNoSpace}),
	)
}
