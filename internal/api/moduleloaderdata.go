package api

import (
	kmmv1beta1 "github.com/rh-ecosystem-edge/kernel-module-management/api/v1beta1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ModuleLoaderData contains all the data needed for succesfull execution of
// Build,Sign, ModuleLoader flow per specific kernel. It is constructed at the begining
// of the Reconciliation loop flow.It contains all the data after merging/resolving all the
// definitions (Build/Sign configuration from KM or from Container, ContainerImage etc').
// From that point on , it is the only structure that is needed as input, no need for
// Module or for KernelMapping
type ModuleLoaderData struct {
	// kernel version
	KernelVersion string
	// Repo secret for DS images
	ImageRepoSecret *v1.LocalObjectReference

	// Selector for DS
	Selector map[string]string

	// Name
	Name string

	// Namspace
	Namespace string

	// service account for DS
	ServiceAccountName string

	// Build contains build instructions.
	Build *kmmv1beta1.Build

	// Sign provides default kmod signing settings
	Sign *kmmv1beta1.Sign

	// ContainerImage is a top-level field
	ContainerImage string

	// Image pull policy.
	ImagePullPolicy v1.PullPolicy

	// Modprobe is a set of properties to customize which module modprobe loads and with which properties.
	Modprobe kmmv1beta1.ModprobeSpec

	// RegistryTLS set the TLS configs for accessing the registry of the module-loader's image.
	RegistryTLS *kmmv1beta1.TLSOptions

	// used for setting the owner field of jobs/buildconfigs
	Owner metav1.Object
}
