/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ClusterSpec defines the desired state of Cluster.
type ClusterSpec struct {
	// Additional Domains are domains that you want to accept for
	// binding clusters for.
	//
	// By default the cluster operator will only allow kubeconfigs whose host
	// matches localhost, 127.0.0.1 and localhost.localstack.cloud.
	//
	// This field allows you to add additional providers hosts
	// that you want to accept kubeconfigs for.
	//
	// +optional
	AdditionalDomains []string `json:"additionalDomains,omitempty"`

	// FirewallFormat is the format of the firewall rules that will be
	// generated.
	//
	// +optional
	// +kubebuilder:default=iptables
	// +kubebuilder:validation:Enum=iptables;nftables;ufw;firewalld;ipfw;pf
	FirewallFormat string `json:"firewallFormat,omitempty"`

	// KubeConfigPath is the path on the controller where the kubeconfig
	// file is mounted.
	//
	// +optional
	KubeConfigPath string `json:"kubeConfigPath,omitempty"`

	// NamespacePrefix is a prefix that will be used to create the
	// namespace for the cluster.
	//
	// +optional
	// +kubebuilder:validation:Pattern=^[a-z0-9-]+$
	// +kubebuilder:default=cluster
	NamespacePrefix string `json:"namespacePrefix,omitempty"`

	// ReconcileInterval is the interval at which the controller will
	// reconcile the cluster.
	//
	// +optional
	// +kubebuilder:default="30s"
	ReconcileInterval metav1.Duration `json:"reconcileInterval,omitempty"`

	// RemapToIp is the IP address that the localhost domain will be
	// remapped to.
	//
	// +required
	// +kubebuilder:validation:Format=ipv4
	RemapToIp string `json:"remapToIp,omitempty"`

	// Suspend will suspend the cluster.
	//
	// +optional
	Suspend bool `json:"suspend,omitempty"`
}

type ClusterStatusEntry struct {
	// Ready is true when the cluster is ready to accept requests.
	Ready bool `json:"ready"`

	// Endpoint is the endpoint of the cluster.
	Endpoint string `json:"endpoint"`

	// KubeConfig is the kubeconfig secret for the cluster.
	KubeConfig string `json:"kubeConfig"`

	// LastUpdateTime is the last time the cluster was updated.
	LastUpdateTime metav1.Time `json:"lastUpdateTime"`
}

type ClusterStatusEntries map[string]ClusterStatusEntry

// ClusterStatus defines the observed state of Cluster.
type ClusterStatus struct {
	// +kubebuilder:pruning:PreserveUnknownFields
	Clusters ClusterStatusEntries `json:"clusters"`

	// FirewallRules are a set of firewall rules that may be required
	// to access remote clusters.
	//
	// +optional
	FirewallRules []string `json:"firewallRules,omitempty"`

	// DeletionRules are a set of firewall rules that may be required
	// to delete the firewall rules created for the cluster(s)
	//
	// +optional
	DeletionRules []string `json:"deletionRules,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Cluster is the Schema for the clusters API.
type Cluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ClusterSpec `json:"spec,omitempty"`

	// Status is the status of the cluster.
	// +kubebuilder:pruning:PreserveUnknownFields
	Status ClusterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ClusterList contains a list of Cluster.
type ClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Cluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Cluster{}, &ClusterList{})
}
