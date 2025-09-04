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

package v1alpha1

import (
	"log"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/conversion"

	devboxv1alpha2 "github.com/labring/sealos/controllers/devbox/api/v1alpha2"
)

// ConvertTo converts this Devbox (v1alpha1) to the Hub version (v1alpha2).
func (src *Devbox) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*devboxv1alpha2.Devbox)
	log.Printf("ConvertTo: Converting Devbox from Spoke version v1alpha1 to Hub version v1alpha2;"+
		"source: %s/%s, target: %s/%s", src.Namespace, src.Name, dst.Namespace, dst.Name)

	// Copy ObjectMeta to preserve name, namespace, labels, etc.
	dst.ObjectMeta = src.ObjectMeta
	// Transform Devbox from v1alpha1 to v1alpha2
	transformDevboxV1alpha1ToV1alpha2(src, dst)
	return nil
}


// ConvertFrom converts the Hub version (v1alpha2) to this Devbox (v1alpha1).
func (dst *Devbox) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*devboxv1alpha2.Devbox)
	log.Printf("ConvertFrom: Converting Devbox from Hub version v1alpha2 to Spoke version v1alpha1;"+
		"source: %s/%s, target: %s/%s", src.Namespace, src.Name, dst.Namespace, dst.Name)

	// Copy ObjectMeta to preserve name, namespace, labels, etc.
	dst.ObjectMeta = src.ObjectMeta
	// Transform Devbox from v1alpha2 to v1alpha1
	transformDevboxV1alpha2ToV1alpha1(src, dst)
	return nil
}

func transformDevboxV1alpha1ToV1alpha2(src *Devbox, dst *devboxv1alpha2.Devbox) {
	// Set TypeMeta
	dst.TypeMeta = metav1.TypeMeta{
		APIVersion: "devbox.sealos.io/v1alpha2",
		Kind:       "Devbox",
	}

	// ObjectMeta is already copied in the calling function

	// Transform Spec
	dst.Spec = devboxv1alpha2.DevboxSpec{
		State:      devboxv1alpha2.DevboxState(src.Spec.State),
		Resource:   src.Spec.Resource,
		Image:      src.Spec.Image,
		TemplateID: src.Spec.TemplateID,
		Config: devboxv1alpha2.Config{
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
		StorageLimit: "10Gi",
		NetworkSpec: devboxv1alpha2.NetworkSpec{
			Type:       devboxv1alpha2.NetworkType(src.Spec.NetworkSpec.Type),
			ExtraPorts: src.Spec.NetworkSpec.ExtraPorts,
		},
		RuntimeClassName: src.Spec.RuntimeClassName,
		NodeSelector:     src.Spec.NodeSelector,
		Tolerations:      src.Spec.Tolerations,
		Affinity:         src.Spec.Affinity,
	}

	// Transform Status
	dst.Status = devboxv1alpha2.DevboxStatus{
		Network: devboxv1alpha2.NetworkStatus{
			Type:     devboxv1alpha2.NetworkType(src.Status.Network.Type),
			NodePort: src.Status.Network.NodePort,
			TailNet:  src.Status.Network.TailNet,
		},
		Phase:         devboxv1alpha2.DevboxPhase(src.Status.Phase),
		State:         devboxv1alpha2.DevboxState(src.Spec.State),
		CommitRecords: transformCommitHistories(src.Status.CommitHistory),
		// Note: v1alpha2 has additional fields ContentID, Node, LastContainerStatus
		// We'll leave them empty as v1alpha1 doesn't have equivalent fields
        // wait for devbox controller to update the status
        // ContentID: "",
        // Node: "",
        // LastContainerStatus: corev1.ContainerStatus{},
	}
}

func transformCommitHistories(commitHistories []*CommitHistory) devboxv1alpha2.CommitRecordMap {
	commitRecordMap := devboxv1alpha2.CommitRecordMap{}
	for _, commitHistory := range commitHistories {
		if commitHistory.ContainerID == "" {
			continue
		}
		commitRecordMap[commitHistory.ContainerID] = transformCommitHistory(commitHistory)
	}
	return commitRecordMap
}

func transformCommitHistory(commitHistory *CommitHistory) *devboxv1alpha2.CommitRecord {
	return &devboxv1alpha2.CommitRecord{
		BaseImage:    "",
		CommitImage:  commitHistory.Image,
		Node:         commitHistory.Node,
		GenerateTime: commitHistory.Time,
		ScheduleTime: commitHistory.Time,
		CommitTime:   commitHistory.Time,
		CommitStatus: devboxv1alpha2.CommitStatus(commitHistory.Status),
	}
}

func transformDevboxV1alpha2ToV1alpha1(src *devboxv1alpha2.Devbox, dst *Devbox) {
	// Set TypeMeta
	dst.TypeMeta = metav1.TypeMeta{
		APIVersion: "devbox.sealos.io/v1alpha1",
		Kind:       "Devbox",
	}

	// ObjectMeta is already copied in the calling function

	// Transform Spec
	dst.Spec = DevboxSpec{
		State:      DevboxState(src.Spec.State),
		Resource:   src.Spec.Resource,
		Image:      src.Spec.Image,
		TemplateID: src.Spec.TemplateID,
		// Note: v1alpha2 has StorageLimit but v1alpha1 has Squash - we'll set Squash to false
		// as there's no direct mapping from StorageLimit to Squash
		Squash: false,
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
	}

	// Transform Status
	dst.Status = DevboxStatus{
		Network: NetworkStatus{
			Type:     NetworkType(src.Status.Network.Type),
			NodePort: src.Status.Network.NodePort,
			TailNet:  src.Status.Network.TailNet,
		},
		Phase:         DevboxPhase(src.Status.Phase),
		CommitHistory: transformCommitRecords(src.Status.CommitRecords),
		// Note: v1alpha1 has State and LastTerminationState fields in status which are corev1.ContainerState
		// v1alpha2 doesn't have these, so we'll leave them empty
		State:                corev1.ContainerState{},
		LastTerminationState: corev1.ContainerState{},
	}
}

func transformCommitRecords(commitRecords devboxv1alpha2.CommitRecordMap) []*CommitHistory {
	var commitHistories []*CommitHistory
	for containerID, commitRecord := range commitRecords {
		if commitRecord == nil {
			continue
		}
		commitHistories = append(commitHistories, &CommitHistory{
			Image:            commitRecord.CommitImage,
			Time:             commitRecord.CommitTime,
			Pod:              "", // v1alpha2 doesn't have Pod field, leave empty
			Status:           CommitStatus(commitRecord.CommitStatus),
			PredicatedStatus: CommitStatus(commitRecord.CommitStatus), // Use same status for both
			Node:             commitRecord.Node,
			ContainerID:      containerID,
		})
	}
	return commitHistories
}
