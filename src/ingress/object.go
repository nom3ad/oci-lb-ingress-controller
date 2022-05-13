package ingress

import (
	networking "k8s.io/api/networking/v1"
)

var OCILoadbalancerIngressClass = "oci"

const (
	KubernetesIngressClassAnnotation = "kubernetes.io/ingress.class"
)

// IsOCILoadbalancerIngress returns true if an ingress object has the OCI ingress class
func IsOCILoadbalancerIngress(ingress *networking.Ingress) bool {
	return GetIngressClassName(ingress) == OCILoadbalancerIngressClass
}

func GetIngressClassName(ingress *networking.Ingress) string {
	if ingress.Spec.IngressClassName != nil {
		return *ingress.Spec.IngressClassName
	}
	if ingressClass, ok := ingress.GetAnnotations()[KubernetesIngressClassAnnotation]; ok {
		return ingressClass
	}
	return ""
}
