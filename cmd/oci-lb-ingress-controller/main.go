package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	providercfg "github.com/nom3ad/oci-lb-ingress-controller/pkg/cloudprovider/providers/oci/config"
	"github.com/nom3ad/oci-lb-ingress-controller/src/configholder"
	"github.com/nom3ad/oci-lb-ingress-controller/src/controller"
	"github.com/nom3ad/oci-lb-ingress-controller/src/ingress"
	"github.com/nom3ad/oci-lb-ingress-controller/version"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func main() {
	fmt.Fprintln(os.Stderr, version.String())
	var logger *zap.Logger
	if strings.ToUpper(os.Getenv("ZAP_DEV_LOGGER")) == "TRUE" {
		logger, _ = zap.NewDevelopment()
	} else {
		logger, _ = zap.NewProduction()
	}
	logger = logger.WithOptions(zap.AddStacktrace(zapcore.ErrorLevel))
	switch strings.ToUpper(os.Getenv("ZAP_LOG_LEVEL")) {
	case "DEBUG":
	case "", "INFO":
		logger = logger.WithOptions(zap.IncreaseLevel(zapcore.InfoLevel))
	case "WARN":
		logger = logger.WithOptions(zap.IncreaseLevel(zapcore.WarnLevel))
	case "ERROR":
		logger = logger.WithOptions(zap.IncreaseLevel(zapcore.ErrorLevel))
	}
	zap.ReplaceGlobals(logger)
	defer logger.Sync()

	configPath := flag.String("config", "config.yml", "Path to config file")
	ingressClass := flag.String("ingress-class", "oci", "Ingress class to be used")
	controllerName := flag.String("controller-name", "ingress.beta.kubernetes.io/oci", "controller name.  must be a domain-prefixed path (such as 'acme.io/foo')")
	defaultLoadBalancerSubnetIds := flag.String("default-subnets", "", "comma separated list of subnet ocids (max=2)")
	defaultLoadBalancerShape := flag.String("default-loadbalancer-shape", "", "Default loadbalancer shape.  eg: 'flexible', '10mbps', '100mbps'")
	defaultFlexShapeMinMbps := flag.Int("default-flexible-shape-min-mbps", 0, "Default minimum bandwidth if loadbalancer shape is 'flexible'")
	defaultFlexShapeMaxMbps := flag.Int("default-flexible-shape-max-mbps", 0, "Default maximum bandwidth if loadbalancer shape is 'flexible'")
	forceHTTPSRedirection := flag.Bool("force-https-redirection", false, "If set HTTPS Redirection will be forced for ingresses by default")

	flag.Parse()

	// Config loading
	logger.Sugar().Info("Reading config from ", *configPath)
	conf, err := providercfg.FromFile(*configPath)
	if err != nil {
		logger.Sugar().Fatalf("Could not load config from %s: %s", *configPath, err)
	}
	logger.Sugar().Debugf("Loaded config: %+v", conf)

	if ingressClass != nil && *ingressClass != "" {
		ingress.OCILoadbalancerIngressClass = *ingressClass
	}
	if forceHTTPSRedirection != nil {
		ingress.ForceHTTPSRedirectionByDefault = *forceHTTPSRedirection
	}
	if controllerName != nil && *controllerName != "" {
		controller.ControllerName = *controllerName
	}
	if defaultLoadBalancerSubnetIds != nil && *defaultLoadBalancerSubnetIds != "" {
		configholder.DefaultLoadBalancerSubnetIds = *defaultLoadBalancerSubnetIds
	}
	if defaultLoadBalancerShape != nil && *defaultLoadBalancerShape != "" {
		ingress.DefaultLBShape = *defaultLoadBalancerShape
	}
	if defaultFlexShapeMinMbps != nil && *defaultFlexShapeMinMbps != 0 {
		ingress.DefaultFlexShapeMinMbps = *defaultFlexShapeMinMbps
	}
	if defaultFlexShapeMaxMbps != nil && *defaultFlexShapeMaxMbps != 0 {
		ingress.DefaultFlexShapeMaxMbps = *defaultFlexShapeMaxMbps
	}

	logger.Sugar().With("OCILoadbalancerIngressClass", ingress.OCILoadbalancerIngressClass, "ControllerName", controller.ControllerName,
		"ForceHTTPSRedirectionByDefault", ingress.ForceHTTPSRedirectionByDefault, "DefaultLoadBalancerSubnetIds", configholder.DefaultLoadBalancerSubnetIds,
		"DefaultLBShape", ingress.DefaultLBShape, "DefaultFlexShapeMinMbps", ingress.DefaultFlexShapeMinMbps,
		"DefaultFlexShapeMaxMbps", ingress.DefaultFlexShapeMaxMbps).Info("Settings")

	// Start ingress controller
	logger.Sugar().With("kubernetes.io/ingress.class", ingress.OCILoadbalancerIngressClass, "controllerName", controller.ControllerName).Infof("Starting ingress controller")
	if err := controller.Run(conf, logger); err != nil {
		logger.Sugar().Fatal("Failed to start controller: ", err)
	}
	logger.Info("Exiting ingress controller")
}
