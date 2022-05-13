package client

import (
	providercfg "github.com/nom3ad/oci-lb-ingress-controller/pkg/cloudprovider/providers/oci/config"
	"go.uber.org/zap"
	"k8s.io/client-go/util/flowcontrol"
)

const (
	rateLimitQPSDefault    = 20.0
	rateLimitBucketDefault = 5
)

// RateLimiter reader and writer.
type RateLimiter struct {
	Reader flowcontrol.RateLimiter
	Writer flowcontrol.RateLimiter
}

// NewRateLimiter builds and returns a struct containing read and write
// rate limiters. Defaults are used where no (0) value is provided.
func NewRateLimiter(logger *zap.SugaredLogger, config *providercfg.RateLimiterConfig) RateLimiter {
	if config == nil {
		config = &providercfg.RateLimiterConfig{}
	}

	//If RateLimiter is disabled we would use FakeAlwaysRateLimiter that always accepts the request
	if config.DisableRateLimiter {
		logger.Info("Cloud Provider OCI rateLimiter is disabled")
		return RateLimiter{
			Reader: flowcontrol.NewFakeAlwaysRateLimiter(),
			Writer: flowcontrol.NewFakeAlwaysRateLimiter(),
		}
	}

	// Set to default values if configuration not declared
	if config.RateLimitQPSRead == 0 {
		config.RateLimitQPSRead = rateLimitQPSDefault
	}
	if config.RateLimitBucketRead == 0 {
		config.RateLimitBucketRead = rateLimitBucketDefault
	}
	if config.RateLimitQPSWrite == 0 {
		config.RateLimitQPSWrite = rateLimitQPSDefault
	}
	if config.RateLimitBucketWrite == 0 {
		config.RateLimitBucketWrite = rateLimitBucketDefault
	}

	rateLimiter := RateLimiter{
		Reader: flowcontrol.NewTokenBucketRateLimiter(
			config.RateLimitQPSRead,
			config.RateLimitBucketRead),
		Writer: flowcontrol.NewTokenBucketRateLimiter(
			config.RateLimitQPSWrite,
			config.RateLimitBucketWrite),
	}

	logger.Infof("OCI using read rate limit configuration: QPS=%g, bucket=%d",
		config.RateLimitQPSRead,
		config.RateLimitBucketRead)

	logger.Infof("OCI using write rate limit configuration: QPS=%g, bucket=%d",
		config.RateLimitQPSWrite,
		config.RateLimitBucketWrite)

	return rateLimiter
}
