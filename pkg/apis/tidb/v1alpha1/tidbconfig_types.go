
// Copyright 2019 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.


package v1alpha1

import (
	"log"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// TidbConfig
// +k8s:openapi-gen=true
// +resource:path=tidbconfigs,strategy=TidbConfigStrategy,rest=TidbConfigREST
type TidbConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TidbConfigSpec   `json:"spec,omitempty"`
	Status TidbConfigStatus `json:"status,omitempty"`
}

// TidbConfigSpec defines the desired state of TidbConfig
type TidbConfigSpec struct {
	Replicas int `json:"replicas,omitempty"`
}

// TidbConfigStatus defines the observed state of TidbConfig
type TidbConfigStatus struct {
	CurrentReplicas int `json:"currentReplicas,omitempty"`
}

// DefaultingFunction sets default TidbConfig field values
func (TidbConfigSchemeFns) DefaultingFunction(o interface{}) {
	obj := o.(*TidbConfig)
	// set default field values here
	if obj.Spec.Replicas == 0 {
		obj.Spec.Replicas = 1
	}
	log.Printf("Defaulting fields for TidbConfig %s\n", obj.Name)
}
