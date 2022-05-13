package controller

import "github.com/nom3ad/oci-lb-ingress-controller/pkg/oci/client"

func isRetriableError(err error) bool {
	if client.IsRetryableError(err) {
		return true
	}
	if err.Error() == "Retry" { // TODO
		return true
	}
	return false
}

func ignoreNonRetriableError(err error) error {
	if !isRetriableError(err) {
		return nil
	}
	return err
}
