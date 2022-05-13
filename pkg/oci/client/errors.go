package client

import (
	"net/http"

	"github.com/oracle/oci-go-sdk/v46/common"
	"github.com/pkg/errors"
)

// HTTP Error Types
const (
	HTTP400RelatedResourceNotAuthorizedOrNotFoundCode = "RelatedResourceNotAuthorizedOrNotFound"
	HTTP400LimitExceededCode                          = "LimitExceeded"
	HTTP401NotAuthenticatedCode                       = "NotAuthenticated"
	HTTP404NotAuthorizedOrNotFoundCode                = "NotAuthorizedOrNotFound"
	HTTP409IncorrectStateCode                         = "IncorrectState"
	HTTP409NotAuthorizedOrResourceAlreadyExistsCode   = "NotAuthorizedOrResourceAlreadyExists"
	HTTP429TooManyRequestsCode                        = "TooManyRequests"
	HTTP500InternalServerErrorCode                    = "InternalServerError"
)

var errNotFound = errors.New("not found")

// IsNotFound returns true if the given error indicates that a resource could
// not be found.
func IsNotFound(err error) bool {
	if err == nil {
		return false
	}
	err = errors.Cause(err)
	if err == errNotFound {
		return true
	}
	serviceErr, ok := common.IsServiceError(err)
	return ok && serviceErr.GetHTTPStatusCode() == http.StatusNotFound
}

func IsServiceLimitError(err error) bool {
	if err == nil {
		return false
	}
	err = errors.Cause(err)
	serviceErr, ok := common.IsServiceError(err)
	return ok && serviceErr.GetCode() == HTTP400LimitExceededCode
}

func IsRetryableError(err error) bool {
	if err == nil {
		return false
	}

	err = errors.Cause(err)
	serviceErr, ok := common.IsServiceError(err)
	if !ok {
		return false
	}
	return isRetryableServiceError(serviceErr)
}

func isRetryableServiceError(serviceErr common.ServiceError) bool {
	return ((serviceErr.GetHTTPStatusCode() == http.StatusBadRequest) && (serviceErr.GetCode() == HTTP400RelatedResourceNotAuthorizedOrNotFoundCode)) ||
		((serviceErr.GetHTTPStatusCode() == http.StatusBadRequest) && (serviceErr.GetCode() == HTTP400LimitExceededCode)) ||
		((serviceErr.GetHTTPStatusCode() == http.StatusUnauthorized) && (serviceErr.GetCode() == HTTP401NotAuthenticatedCode)) ||
		((serviceErr.GetHTTPStatusCode() == http.StatusNotFound) && (serviceErr.GetCode() == HTTP404NotAuthorizedOrNotFoundCode)) ||
		((serviceErr.GetHTTPStatusCode() == http.StatusConflict) && (serviceErr.GetCode() == HTTP409IncorrectStateCode)) ||
		((serviceErr.GetHTTPStatusCode() == http.StatusConflict) && (serviceErr.GetCode() == HTTP409NotAuthorizedOrResourceAlreadyExistsCode)) ||
		((serviceErr.GetHTTPStatusCode() == http.StatusTooManyRequests) && (serviceErr.GetCode() == HTTP429TooManyRequestsCode)) ||
		((serviceErr.GetHTTPStatusCode() == http.StatusInternalServerError) && (serviceErr.GetCode() == HTTP500InternalServerErrorCode))
}
