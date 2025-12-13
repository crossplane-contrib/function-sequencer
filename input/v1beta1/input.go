// Package v1beta1 contains the input type for this Function
// +kubebuilder:object:generate=true
// +groupName=sequencer.fn.crossplane.io
// +versionName=v1beta1
package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/crossplane/function-sdk-go/resource"
)

// This isn't a custom resource, in the sense that we never install its CRD.
// It is a KRM-like object, so we generate a CRD to describe its schema.

// SequencingRule is a rule that describes a sequence of resources.
type SequencingRule struct {
	// TODO: Should we add a way to infer sequencing from usages? e.g. InferFromUsages: true
	// InferFromUsages bool            `json:"inferFromUsages,omitempty"`

	// Sequence is a list of composition resource names.
	Sequence []resource.Name `json:"sequence,omitempty"`
}

// UsageVersion defines the version of the Usage resource.
type UsageVersion string

const (
	// UsageV1 indicates that Crossplane v1 apiextensions Usage should be used.
	UsageV1 UsageVersion = "v1"

	// UsageV2 indicates that Crossplane v2 protection Usage should be user.
	UsageV2 UsageVersion = "v2"
)

// Input can be used to provide input to this Function.
// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:resource:categories=crossplane
type Input struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// EnableDeletionSequencing controls the automatic creation of Usage/ClusterUsage resources from the dependency tree
	// defined by the rule sequences.
	// +kubebuilder:object:default=false
	EnableDeletionSequencing bool `json:"enableDeletionSequencing,omitempty"`
	// ReplayDeletion sets the Usage/ClusterUsage replayDeletion attribute.
	// +kubebuilder:object:default=true
	ReplayDeletion bool `json:"replayDeletion,omitempty"`
	// UsageVersion specifies the version of Usage/ClusterUsage resource to be created.
	// +kubebuilder:object:default="v2"
	UsageVersion UsageVersion `json:"usageVersion,omitempty"`

	// ResetCompositeReadiness sets the composite ready state to false if desired resources are removed from the request.
	// +kubebuilder:object:default=false
	ResetCompositeReadiness bool `json:"resetCompositeReadiness,omitempty"`
	// Rules is a list of rules that describe sequences of resources.
	Rules []SequencingRule `json:"rules"`
}
