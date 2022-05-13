package oci

import (
	providercfg "github.com/nom3ad/oci-lb-ingress-controller/pkg/cloudprovider/providers/oci/config"
	"github.com/nom3ad/oci-lb-ingress-controller/pkg/oci/client"
	"go.uber.org/zap"
)

type CloudProvider struct {
	// NodeLister listersv1.NodeLister

	client client.Interface
	// kubeclient clientset.Interface

	// securityListManagerFactory securityListManagerFactory
	config *providercfg.Config

	logger *zap.SugaredLogger
	// instanceCache cache.Store
	// metricPusher  *metrics.MetricPusher
}

func DummyCp(client client.Interface, config *providercfg.Config, logger *zap.SugaredLogger) *CloudProvider {
	return &CloudProvider{
		client: client,
		logger: logger,
		config: config,
	}
}
