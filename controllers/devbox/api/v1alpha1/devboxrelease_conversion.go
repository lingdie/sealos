package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/conversion"

	devboxv1alpha2 "github.com/labring/sealos/controllers/devbox/api/v1alpha2"
)

// ConvertTo converts this DevBoxRelease (v1alpha1) to the Hub version (v1alpha2).
func (src *DevBoxRelease) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*devboxv1alpha2.DevBoxRelease)

	// Copy ObjectMeta to preserve name, namespace, labels, etc.
	dst.ObjectMeta = src.ObjectMeta

	// Set TypeMeta for the target version: v1alpha2
	dst.TypeMeta = metav1.TypeMeta{
		APIVersion: "devbox.sealos.io/v1alpha2",
		Kind:       "DevBoxRelease",
	}
	dst.OwnerReferences = make([]metav1.OwnerReference, len(src.OwnerReferences))
	copy(dst.OwnerReferences, src.OwnerReferences)
	for i := range dst.OwnerReferences {
		dst.OwnerReferences[i].APIVersion = "devbox.sealos.io/v1alpha2"
	}

	// Transform DevBoxRelease from v1alpha1 to v1alpha2
	dst.Spec.DevboxName = src.Spec.DevboxName
	dst.Spec.Version = src.Spec.NewTag
	dst.Spec.Notes = src.Spec.Notes
	dst.Status.OriginalDevboxState = ""
	dst.Status.Phase = devboxv1alpha2.DevBoxReleasePhase(src.Status.Phase)
	dst.Status.SourceImage = src.Status.OriginalImage
	dst.Status.TargetImage = ""
	return nil
}

func (dst *DevBoxRelease) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*devboxv1alpha2.DevBoxRelease)

	// Copy ObjectMeta to preserve name, namespace, labels, etc.
	dst.ObjectMeta = src.ObjectMeta

	// Set TypeMeta for the target version: v1alpha1
	dst.TypeMeta = metav1.TypeMeta{
		APIVersion: "devbox.sealos.io/v1alpha1",
		Kind:       "DevBoxRelease",
	}
	dst.OwnerReferences = make([]metav1.OwnerReference, len(src.OwnerReferences))
	copy(dst.OwnerReferences, src.OwnerReferences)
	for i := range dst.OwnerReferences {
		dst.OwnerReferences[i].APIVersion = "devbox.sealos.io/v1alpha1"
	}

	// Transform DevBoxRelease from v1alpha2 to v1alpha1
	dst.Spec.DevboxName = src.Spec.DevboxName
	dst.Spec.NewTag = src.Spec.Version
	dst.Spec.Notes = src.Spec.Notes
	dst.Status.OriginalImage = src.Status.SourceImage
	dst.Status.Phase = DevBoxReleasePhase(src.Status.Phase)
	return nil
}
