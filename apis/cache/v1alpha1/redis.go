package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
)

// RedisParameters define the specific parameters for a Redis instance
type RedisParameters struct {
	// PasswordSecretRef references the secret that contains the password used
	// in the creation of this redis instance. If no reference is given, the
	// redis instance will be exposed with no password.
	// +optional
	// +immutable
	PasswordSecretRef *v1.SecretKeySelector `json:"passwordSecretRef,omitempty"`

	// ConfigVariables allows users to put arbitrary environment variables
	// for the redis instance.
	// +optional
	// +immutable
	ConfigVariables map[string]string `json:"redisConf,omitempty"`

	// MemoryLimit states the maximum amount of memory available
	// to the pod in the deployment
	// +optional
	// +immutable
	MemoryLimit *string `json:"memoryLimit,omitempty"`
}

// An RedisSpec defines the desired state of a Redis instance.
type RedisSpec struct {
	v1.ResourceSpec `json:",inline"`
	ForProvider     RedisParameters `json:"forProvider"`
}

// RedisExternalStatus keeps the state for the external resource
type RedisExternalStatus struct {
}

// An RedisStatus represents the observed state of a Redis instance.
type RedisStatus struct {
	v1.ResourceStatus `json:",inline"`
	AtProvider        RedisExternalStatus `json:"atProvider"`
}

// +kubebuilder:object:root=true

// Redis is a managed resource that represents a Redis cache.
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories={crossplane,managed,incluster}
type Redis struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RedisSpec   `json:"spec"`
	Status RedisStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// RedisList contains a list of Redis databases
type RedisList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Redis `json:"items"`
}
