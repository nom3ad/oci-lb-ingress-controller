package ingress

import networking "k8s.io/api/networking/v1"

func validateIngress(ing *networking.Ingress) error {
	// TODO:
	// if err := validateProtocols(svc.Spec.Ports); err != nil {
	// 	return err
	// }

	// if svc.Spec.SessionAffinity != corev1.ServiceAffinityNone {
	// 	return errors.New("OCI only supports SessionAffinity \"None\" currently")
	// }

	return nil
}
