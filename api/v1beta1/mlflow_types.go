/*


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

package v1beta1

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// MlflowSpec defines the desired state of Mlflow
type MlflowSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
	// +optional
	Source string `json:"source,omitempty"`

	ArtifactRoot string `json:"artifactroot,omitempty"`
	// AKSK to mount FDS
	Env []corev1.EnvVar `json:"env,omitempty"`

	// List of volumes that can be mounted by containers belonging to the pod.
	// More info: https://kubernetes.io/docs/concepts/storage/volumes
	// +optional
	Volumes []corev1.Volume `json:"volumes,omitempty"`
	// Pod volumes to mount into the container's filesystem.
	// +optional
	VolumeMounts []corev1.VolumeMount `json:"volumeMounts,omitempty"`

	// NOTE(xychu): use affinity to support node selector and other placement requirments
	// +optional
	Affinity *corev1.Affinity `json:"affinity,omitempty"`
}

// MlflowStatus defines the observed state of Mlflow
type MlflowStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Condition appsv1.DeploymentCondition `json:"condition,omitempty"`
	URI       string                     `json:"uri,omitempty"`

	TrainsRecord map[string][]string `json:"trainsrecord,omitempty"`
	Experiments  []string            `json:"experiments,omitempty"`
}

// +kubebuilder:object:root=true

// Mlflow is the Schema for the mlflows API
type Mlflow struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MlflowSpec   `json:"spec,omitempty"`
	Status MlflowStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// MlflowList contains a list of Mlflow
type MlflowList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Mlflow `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Mlflow{}, &MlflowList{})
}
