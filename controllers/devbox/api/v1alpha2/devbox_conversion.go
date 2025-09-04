/*
Copyright 2024.

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

package v1alpha2

import (
	"log"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/conversion"

	devboxv1alpha1 "github.com/labring/sealos/controllers/devbox/api/v1alpha1"
)

// ConvertTo converts this Devbox (v1alpha2) to the Hub version (v1alpha1).
func (src *Devbox) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*devboxv1alpha1.Devbox)
	log.Printf("ConvertTo: Converting Devbox from Spoke version v1alpha2 to Hub version v1alpha1;"+
		"source: %s/%s, target: %s/%s", src.Namespace, src.Name, dst.Namespace, dst.Name)

	// Copy ObjectMeta to preserve name, namespace, labels, etc.
	dst.ObjectMeta = src.ObjectMeta

	return nil
}

// ConvertFrom converts the Hub version (v1alpha1) to this Devbox (v1alpha2).
func (dst *Devbox) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*devboxv1alpha1.Devbox)
	log.Printf("ConvertFrom: Converting Devbox from Hub version v1alpha1 to Spoke version v1alpha2;"+
		"source: %s/%s, target: %s/%s", src.Namespace, src.Name, dst.Namespace, dst.Name)

	// Copy ObjectMeta to preserve name, namespace, labels, etc.
	transformDevboxV1alpha1ToV1alpha2(src, dst)
	return nil
}

func transformDevboxV1alpha1ToV1alpha2(src *devboxv1alpha1.Devbox, dst *Devbox) {

	dst.ObjectMeta = src.ObjectMeta

	dst = &Devbox{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "devbox.sealos.io/v1alpha2",
			Kind:       "Devbox",
		},
		ObjectMeta: src.ObjectMeta,
		Spec: DevboxSpec{
			State:      DevboxState(src.Spec.State),
			Resource:   src.Spec.Resource,
			Image:      src.Spec.Image,
			TemplateID: src.Spec.TemplateID,
			Config: Config{
				User:           src.Spec.Config.User,
				Labels:         src.Spec.Config.Labels,
				Annotations:    src.Spec.Config.Annotations,
				Command:        src.Spec.Config.Command,
				Args:           src.Spec.Config.Args,
				WorkingDir:     src.Spec.Config.WorkingDir,
				Env:            src.Spec.Config.Env,
				ReleaseCommand: src.Spec.Config.ReleaseCommand,
				ReleaseArgs:    src.Spec.Config.ReleaseArgs,
				Ports:          src.Spec.Config.Ports,
				AppPorts:       src.Spec.Config.AppPorts,
				VolumeMounts:   src.Spec.Config.VolumeMounts,
				Volumes:        src.Spec.Config.Volumes,
			},
			NetworkSpec: NetworkSpec{
				Type:       NetworkType(src.Spec.NetworkSpec.Type),
				ExtraPorts: src.Spec.NetworkSpec.ExtraPorts,
			},
			RuntimeClassName: src.Spec.RuntimeClassName,
			NodeSelector:     src.Spec.NodeSelector,
			Tolerations:      src.Spec.Tolerations,
			Affinity:         src.Spec.Affinity,
		},
		Status: DevboxStatus{
			Network: NetworkStatus{
				Type:     NetworkType(src.Status.Network.Type),
				NodePort: src.Status.Network.NodePort,
				TailNet:  src.Status.Network.TailNet,
			},
			Phase:         DevboxPhase(src.Status.Phase),
			State:         DevboxState(src.Spec.State),
			CommitRecords: transformCommitHistories(src.Status.CommitHistory),
		},
	}
}

func transformCommitHistories(commitHistories []*devboxv1alpha1.CommitHistory) CommitRecordMap {
	commitRecordMap := CommitRecordMap{}
	for _, commitHistory := range commitHistories {
		if commitHistory.ContainerID == "" {
			continue
		}
		commitRecordMap[commitHistory.ContainerID] = transformCommitHistory(commitHistory)
	}
	return commitRecordMap
}

func transformCommitHistory(commitHistory *devboxv1alpha1.CommitHistory) *CommitRecord {
	return &CommitRecord{
		BaseImage:    "",
		CommitImage:  commitHistory.Image,
		Node:         commitHistory.Node,
		GenerateTime: commitHistory.Time,
		ScheduleTime: commitHistory.Time,
		CommitTime:   commitHistory.Time,
		CommitStatus: CommitStatus(commitHistory.Status),
	}
}
