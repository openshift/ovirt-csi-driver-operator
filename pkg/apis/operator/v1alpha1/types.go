package v1alpha1

import (
	operatorv1 "github.com/openshift/api/operator/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// OvirtCSIDriver is a specification for a OvirtCSIDriver resource
type OvirtCSIDriver struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OvirtCSIDriverSpec   `json:"spec"`
	Status OvirtCSIDriverStatus `json:"status"`
}

// OvirtCSIDriverSpec is the spec for a OvirtCSIDriver resource
type OvirtCSIDriverSpec struct {
	operatorv1.OperatorSpec `json:",inline"`
}

// OvirtCSIDriverStatus is the status for a OvirtCSIDriver resource
type OvirtCSIDriverStatus struct {
	operatorv1.OperatorStatus `json:",inline"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// OvirtCSIDriverList is a list of OvirtCSIDriver resources
type OvirtCSIDriverList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []OvirtCSIDriver `json:"items"`
}
