package v1alpha1

import (
	"log"

	devboxv1alpha2 "github.com/labring/sealos/controllers/devbox/api/v1alpha2"

	"sigs.k8s.io/controller-runtime/pkg/conversion"
)

// ConvertTo converts this DevBoxRelease (v1alpha1) to the Hub version (v1alpha2).
func (src *DevBoxRelease) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*devboxv1alpha2.DevBoxRelease)
	log.Printf("ConvertTo: Converting DevBoxRelease from Spoke version v1alpha1 to Hub version v1alpha2;"+
		"source: %s/%s, target: %s/%s", src.Namespace, src.Name, dst.Namespace, dst.Name)
	return nil
}

func (dst *DevBoxRelease) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*devboxv1alpha2.DevBoxRelease)
	log.Printf("ConvertFrom: Converting DevBoxRelease from Hub version v1alpha2 to Spoke version v1alpha1;"+
		"source: %s/%s, target: %s/%s", src.Namespace, src.Name, dst.Namespace, dst.Name)
	return nil
}
