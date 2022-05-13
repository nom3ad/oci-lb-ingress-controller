package controller

import (
	"github.com/nom3ad/oci-lb-ingress-controller/pkg/cloudprovider/providers/oci"
	providercfg "github.com/nom3ad/oci-lb-ingress-controller/pkg/cloudprovider/providers/oci/config"
	"github.com/nom3ad/oci-lb-ingress-controller/pkg/oci/client"
	"github.com/nom3ad/oci-lb-ingress-controller/src/configholder"
	"github.com/nom3ad/oci-lb-ingress-controller/src/controller/handlers"
	ingressmanager "github.com/nom3ad/oci-lb-ingress-controller/src/manager"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	networking "k8s.io/api/networking/v1"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/builder#example-Builder

var ControllerName = "ingress.beta.kubernetes.io/oci" // must be a domain-prefixed path (such as "acme.io/foo")

func Run(conf *providercfg.Config, logger *zap.Logger) error {
	cp, err := providercfg.NewConfigurationProvider(conf)
	if err != nil {
		return errors.Wrap(err, "Couldn't load OCI config provider")
	}
	rateLimiter := client.NewRateLimiter(logger.Sugar(), conf.RateLimiter)
	ociClient, err := client.New(logger.Sugar(), cp, &rateLimiter)
	if err != nil {
		return errors.Wrap(err, "Couldn't construct OCI client")
	}
	// K8s stuff
	// https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/manager#example-New
	kubeConfig, err := config.GetConfig()
	if err != nil {
		return errors.Wrap(err, "Unable get k8s config")
	}
	controllerMgr, err := manager.New(kubeConfig, manager.Options{})
	if err != nil {
		return errors.Wrap(err, "Unable to set up controller manager")
	}
	dummyCp := oci.DummyCp(ociClient, conf, logger.Sugar())
	ociIngressManager := ingressmanager.New(ociClient, configholder.NewConfigHolder(conf), controllerMgr, dummyCp, logger)
	reconciler, err := NewReconciler(controllerMgr, ociIngressManager, logger)
	if err != nil {
		return errors.Wrap(err, "Couldn't build reconciler")
	}

	c, err := controller.New(ControllerName, controllerMgr, controller.Options{Reconciler: reconciler})
	if err != nil {
		return errors.Wrap(err, "Couldn't build controller")
	}

	if err := setupEventListeners(c, controllerMgr.GetCache(), logger); err != nil {
		if err != nil {
			return errors.Wrap(err, "Couldn't setup event listeners")
		}
	}

	if err := controllerMgr.Start(signals.SetupSignalHandler()); err != nil {
		return errors.Wrap(err, "Couldn't start controller listeners")
	}
	return nil
}

func setupEventListeners(c controller.Controller, cache cache.Cache, logger *zap.Logger) error {
	// Watch Ingress objects for changes (Create, Update, Delete)
	if err := c.Watch(&source.Kind{Type: &networking.Ingress{}}, handlers.NewIngressEventHandler(cache, logger)); err != nil {
		return err
	}

	// Watch Node objects for changes (Create, Delete)
	if err := c.Watch(&source.Kind{Type: &corev1.Node{}}, handlers.NewNodeEventHandler(cache, logger)); err != nil {
		return err
	}

	// Watch Secret objects for changes (Update)
	if err := c.Watch(&source.Kind{Type: &corev1.Secret{}}, handlers.NewSecretEventHandler(cache, logger)); err != nil {
		return err
	}
	return nil
}
