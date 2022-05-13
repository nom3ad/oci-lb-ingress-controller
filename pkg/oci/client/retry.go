package client

import (
	"math"
	"time"

	"github.com/oracle/oci-go-sdk/v46/common"
	"github.com/pkg/errors"
)

// RateLimitError produces an Errorf for rate limiting.
func RateLimitError(isWrite bool, opName string) error {
	opType := "read"
	if isWrite {
		opType = "write"
	}
	return errors.Errorf("rate limited(%s) for operation: %s", opType, opName)
}

func newRetryPolicy() *common.RetryPolicy {
	return NewRetryPolicyWithMaxAttempts(uint(2))
}

// NewRetryPolicyWithMaxAttempts returns a RetryPolicy with the specified max retryAttempts
func NewRetryPolicyWithMaxAttempts(retryAttempts uint) *common.RetryPolicy {
	isRetryableOperation := func(r common.OCIOperationResponse) bool {
		return IsRetryableError(r.Error)
	}

	nextDuration := func(r common.OCIOperationResponse) time.Duration {
		// you might want wait longer for next retry when your previous one failed
		// this function will return the duration as:
		// 1s, 2s, 4s, 8s, 16s, 32s, 64s etc...
		return time.Duration(math.Pow(float64(2), float64(r.AttemptNumber-1))) * time.Second
	}

	policy := common.NewRetryPolicy(
		retryAttempts, isRetryableOperation, nextDuration,
	)
	return &policy
}
