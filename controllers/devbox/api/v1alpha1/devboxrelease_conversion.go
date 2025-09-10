package v1alpha1

import (
	devboxv1alpha2 "github.com/labring/sealos/controllers/devbox/api/v1alpha2"

	"sigs.k8s.io/controller-runtime/pkg/conversion"
)

// ConvertTo converts this DevBoxRelease (v1alpha1) to the Hub version (v1alpha2).
func (src *DevBoxRelease) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*devboxv1alpha2.DevBoxRelease)
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
	dst.Spec.DevboxName = src.Spec.DevboxName
	dst.Spec.NewTag = src.Spec.Version
	dst.Spec.Notes = src.Spec.Notes
	dst.Status.OriginalImage = src.Status.SourceImage
	dst.Status.Phase = DevBoxReleasePhase(src.Status.Phase)
	return nil
}
