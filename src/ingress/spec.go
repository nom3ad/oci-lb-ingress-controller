package ingress

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"strings"

	"github.com/nom3ad/oci-lb-ingress-controller/pkg/cloudprovider/providers/oci"
	. "github.com/nom3ad/oci-lb-ingress-controller/pkg/cloudprovider/providers/oci"
	ociclient "github.com/nom3ad/oci-lb-ingress-controller/pkg/oci/client"
	"github.com/nom3ad/oci-lb-ingress-controller/pkg/oci/instance/metadata"
	"github.com/nom3ad/oci-lb-ingress-controller/src/configholder"
	"github.com/nom3ad/oci-lb-ingress-controller/src/utils"
	"github.com/oracle/oci-go-sdk/v46/loadbalancer"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	networking "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/types"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

var ForceHTTPSRedirectionByDefault bool

type IngressLBSpec struct {
	oci.LBSpec

	Ingress                *networking.Ingress
	Services               map[string]*corev1.Service
	RoutingPolicies        map[string]loadbalancer.RoutingPolicy // to no ptrs
	RuleSets               map[string]loadbalancer.RuleSetDetails
	HostnameDetails        map[string]loadbalancer.HostnameDetails
	Certificates           map[string]loadbalancer.CertificateDetails
	_serviceAndNodeMapping map[string]map[string]corev1.Node
	//unused stuff from lbspec
	// service *v1.Service
	// nodes   []*v1.Node
}

func (igs *IngressLBSpec) NodesForService(svcName string) []*corev1.Node {
	var nodeList []*corev1.Node
	if nodes, found := igs._serviceAndNodeMapping[svcName]; found {
		for _, n := range nodes {
			_n := n // ! important, you know why.
			nodeList = append(nodeList, &_n)
		}
	}
	return nodeList
}

// func (igs *IngressLBSpec) GetRoutingPolicyNameForListener(listener string) string {
// 	return igs._listenerAssociations[listener].routingPolicy
// }

// func (igs *IngressLBSpec) GetListenersAssociatedWithPolicy(policyName string) []string {
// 	var listeners []string
// 	for lName, assoc := range igs._listenerAssociations {
// 		if assoc.routingPolicy == policyName {
// 			listeners = append(listeners, lName)
// 		}
// 	}
// 	return listeners
// }

// func (igs *IngressLBSpec) GetListenersAssociatedWithHostnameName(hostnameName string) []string {
// 	var listeners []string
// 	for lName, assoc := range igs._listenerAssociations {
// 		if assoc.hostnameName == hostnameName {
// 			listeners = append(listeners, lName)
// 		}
// 	}
// 	return listeners
// }

func (igs *IngressLBSpec) IsFlexibleShape() bool {
	return igs.Shape == FlexibleShapeName
}

func NewIngressLBSpec(config configholder.ConfigHolder, ing *networking.Ingress, ociClient ociclient.Interface, k8sClient k8sclient.Client, logger *zap.Logger) (*IngressLBSpec, error) {
	if err := validateIngress(ing); err != nil {
		return nil, errors.Wrap(err, "invalid ingress")
	}

	internal, err := IsInternalLB(ing)
	if err != nil {
		return nil, err
	}

	shape, flexShapeMinMbps, flexShapeMaxMbps, err := getLBShape(ing)
	if err != nil {
		return nil, err
	}

	ctx := context.Background()

	serviceAndNodeMapping := map[string]map[string]corev1.Node{}

	if err != nil {
		return nil, err
	}

	nodeList := &corev1.NodeList{}
	if err := k8sClient.List(ctx, nodeList); err != nil {
		return nil, errors.Wrapf(err, "Couldn't list nodes")
	}

	hostnameDetailsCollection := map[string]loadbalancer.HostnameDetails{}
	getOrCreateHostnameDetails := func(hostname string) *loadbalancer.HostnameDetails {
		if hostname == "" {
			return nil
		}
		hostnameName := getHostnameName(hostname)
		if details, found := hostnameDetailsCollection[hostnameName]; found {
			return &details
		}
		details := loadbalancer.HostnameDetails{
			Name:     utils.PtrToString(hostnameName),
			Hostname: utils.PtrToString(hostname),
		}
		hostnameDetailsCollection[hostnameName] = details
		return &details
	}

	certificateCollection := map[string]loadbalancer.CertificateDetails{}
	// sslConfigDetailsCollection := map[string]loadbalancer.SslConfigurationDetails{}
	getOrCreateSSLConfigDetails := func(hostname, secretName string) (*loadbalancer.SslConfigurationDetails, error) {
		if hostname == "" || secretName == "" {
			return nil, errors.New("empty hostname or secretName")
		}
		secret := corev1.Secret{}
		secretNsName := utils.AsNamespacedName(secretName, ing.Namespace)
		if err := k8sClient.Get(ctx, secretNsName, &secret); err != nil {
			return nil, errors.Wrapf(err, "Could not get secret %s", secretNsName)
		}
		if secret.Type != corev1.SecretTypeTLS {
			// TODO: support any generic secrets with proper keys
			return nil, errors.Errorf("secret %s is not of type TLS", secretName)
		}
		publicCertStr := string(secret.Data[corev1.TLSCertKey])
		privateKeyStr := string(secret.Data[corev1.TLSPrivateKeyKey])
		block, _ := pem.Decode([]byte(publicCertStr))
		if block == nil {
			return nil, errors.Errorf("Failed to decode tls certificate from secret %s", secretName)
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, errors.Wrapf(err, "Failed to parse tls certificate from secret %s", secretName)
		}
		certificateName := strings.ReplaceAll(secretNsName.String(), "/", "_") + "_" + utils.ByteAlphaNumericDigest(cert.Signature, 22)
		if _, found := certificateCollection[certificateName]; !found {
			cert := loadbalancer.CertificateDetails{
				CertificateName:   utils.PtrToString(certificateName),
				PublicCertificate: &publicCertStr,
				PrivateKey:        &privateKeyStr,
				// CaCertificate: *string,
				// Passphrase: *string,
				// TODO: support caCert and password, and validate data
			}
			certificateCollection[certificateName] = cert

		}
		details := loadbalancer.SslConfigurationDetails{
			CertificateName: &certificateName,
			VerifyDepth:     utils.PtrToInt(1), //default value is not 0
			// VerifyDepth: *int,
			// VerifyPeerCertificate: *bool,
			// CipherSuiteName: *string,
			// ServerOrderPreference: loadbalancer.SslConfigurationDetailsServerOrderPreferenceEnum,
			// Protocols: string,
		}
		// sslConfigDetailsCollection[certificateName] = details
		return &details, nil
	}

	namespace := ing.Namespace
	routePolicies := map[string]loadbalancer.RoutingPolicy{}
	ruleSets := map[string]loadbalancer.RuleSetDetails{}
	listeners := make(map[string]loadbalancer.ListenerDetails)
	services := map[string]*corev1.Service{}

	hostsWithTLS := map[string]networking.IngressTLS{}
	for _, ingTLS := range ing.Spec.TLS {
		for _, host := range ingTLS.Hosts {
			hostsWithTLS[host] = ingTLS
		}
	}

	processBackendSpec := func(backend networking.IngressBackend) (backendSetName string, err error) {
		if backend.Resource != nil {
			return "", errors.New("Backend.Resource not supported")
		}
		svcName := backend.Service.Name
		svcPort := backend.Service.Port.Number
		svcPortName := backend.Service.Port.Name
		svcNsName := types.NamespacedName{Namespace: namespace, Name: svcName}
		svc, found := services[svcName]
		if !found {
			svc = &corev1.Service{}
			if err := k8sClient.Get(ctx, svcNsName, svc); err != nil {
				return "", errors.Wrapf(err, "Could not find service %q", svcNsName)
			}
			services[svcName] = svc
		}
		nodePort := -1
		for _, servicePort := range svc.Spec.Ports {
			if servicePort.Protocol != corev1.ProtocolTCP {
				continue
			}
			if svcPort == 0 {
				if servicePort.Name != svcPortName {
					continue
				}
				svcPort = servicePort.Port
			} else if servicePort.Port != svcPort {
				continue
			}
			if servicePort.NodePort > 0 {
				nodePort = int(servicePort.NodePort)
			}
		}
		if nodePort == -1 {
			return "", errors.Errorf("Could not find NodePort for service %q (Type=%s Ports=%s) for BackendPort: %s %d ",
				svcNsName, svc.Spec.Type, utils.Jsonify(svc.Spec.Ports), svcPortName, svcPort)
		}
		nodes := map[string]corev1.Node{}
		for _, node := range nodeList.Items {
			nodes[node.Name] = node
		}
		serviceAndNodeMapping[svcName] = nodes

		backendSetName = oci.GetBackendSetName(svcName, string(corev1.ProtocolTCP), int(svcPort))
		return backendSetName, nil
	}

	for _, ingRule := range ing.Spec.Rules {
		host := ingRule.Host
		httpRoutingRules := []loadbalancer.RoutingRule{}
		for _, ingPath := range ingRule.HTTP.Paths {
			backend := ingPath.Backend
			backendSetName, err := processBackendSpec(backend)
			if err != nil {
				return nil, err
			}
			routingRule, err := createRoutingRule(ingPath, backendSetName, host)
			if err != nil {
				return nil, errors.Wrapf(err, "Could not deduce routing rule. host: %s | backendSet: %s | path: %v", host, backendSetName, ingPath)
			}
			if utils.ContainsMatching(httpRoutingRules, func(r loadbalancer.RoutingRule) bool { return *r.Name == *routingRule.Name }) {
				logger.Sugar().With("host", host, "routingPolicyRule", routingRule.Name).Warn("Ignoring duplicate routing policy")
			} else {
				httpRoutingRules = append(httpRoutingRules, *routingRule)
			}
		}

		var sSlConfigDetails *loadbalancer.SslConfigurationDetails
		if ingTls, exists := hostsWithTLS[host]; exists {
			sSlConfigDetails, err = getOrCreateSSLConfigDetails(host, ingTls.SecretName)
			if err != nil {
				return nil, errors.Wrapf(err, "Could not build SSL config for host:%q with secret %q", host, ingTls.SecretName)
			}
		}

		hostnameDetails := getOrCreateHostnameDetails(host)

		listenerName, listener := createListenerDetails(ing, hostnameDetails, sSlConfigDetails)

		routingPolicyName := getRoutingPolicyName(host)
		httpRoutingPolicy := loadbalancer.RoutingPolicy{
			Name:                     utils.PtrToString(routingPolicyName),
			ConditionLanguageVersion: loadbalancer.RoutingPolicyConditionLanguageVersionV1,
			Rules:                    httpRoutingRules,
		}
		if samePolicy, exists := routePolicies[routingPolicyName]; exists {
			logger.Sugar().With("host", host, "routingPolicyName", routingPolicyName).Debug("Merging routing policies")
			httpRoutingPolicy.Rules = append(samePolicy.Rules, httpRoutingPolicy.Rules...)
		}
		routePolicies[routingPolicyName] = httpRoutingPolicy
		listener.RoutingPolicyName = &routingPolicyName

		listeners[listenerName] = listener
	}

	// httpBackend = httpBackend
	if ing.Spec.DefaultBackend != nil {
		// From OCI docs:  https://docs.oracle.com/en-us/iaas/Content/Balance/Tasks/hostname_management.htm
		// LB Default Listener
		// If a listener has no virtual hostname specified, that listener is the default for the assigned port.
		// If all listeners on a port have virtual hostnames, the first virtual hostname configured for that port serves as the default listener.

		backendSetName, err := processBackendSpec(*ing.Spec.DefaultBackend)
		if err != nil {
			return nil, err
		}

		listenerName, listener := createDefaultBackendListenerDetails(backendSetName)
		listeners[listenerName] = listener

		defaultBackendRoutingRule, err := createDeafultBackendRoutingRule(backendSetName)
		if err != nil {
			return nil, err
		}
		logger.Sugar().Debugf("Inserting defaultBackendRoutingRule to existing host+path routePolices: %v", utils.StringKeys(routePolicies).List())
		for policyName, routePolicy := range routePolicies {
			routePolicy.Rules = append(routePolicy.Rules, *defaultBackendRoutingRule)
			routePolicies[policyName] = routePolicy // ensure in-place change
		}

		// TODO: should we add defaultBackend listener for HTTPS?
	} else {
		listenerName, listener := createSansVirtualHostListenerDetails()
		listeners[listenerName] = listener
		// TODO: should we add sans-virtualhost listener for HTTPS?
	}

	if len(hostsWithTLS) > 0 && (GetAnnotationWithLowercase(ing, AnnotationForceHTTPSRedirect) == "true" || (GetAnnotationWithLowercase(ing, AnnotationForceHTTPSRedirect) == "" && ForceHTTPSRedirectionByDefault)) {
		ruleSetName, httpRedirectorRuleSet, listenerName, httpRedirectorListener := createListenerDetailsAndRulesetDetailsForHTTPSRedirect(utils.StringKeys(hostsWithTLS).List())
		ruleSets[ruleSetName] = httpRedirectorRuleSet
		listeners[listenerName] = httpRedirectorListener
	}

	subnetIds, err := getLoadBalancerSubnetIds(config, ing, ociClient, logger)
	if err != nil {
		return nil, err
	}

	loadbalancerIP, err := GetLoadBalancerIP(ing)
	if err != nil {
		return nil, err
	}

	lbspec := LBSpec{
		Name:           GetLoadBalancerName(ing.Namespace, ing.Name),
		Subnets:        subnetIds,
		Shape:          shape,
		FlexMin:        flexShapeMinMbps,
		FlexMax:        flexShapeMaxMbps,
		Internal:       internal,
		Listeners:      listeners,
		LoadBalancerIP: loadbalancerIP,

		// Ports: ports,
		// SSLConfig: sslConfig,
		// SourceCIDRs:             sourceCIDRs,
		// NetworkSecurityGroupIds: networkSecurityGroupIds,
		// Nodes:               nodes, // TODO
		SecurityListManager: oci.NewSecurityListManagerNOOP(), // TODO
	}
	spec := &IngressLBSpec{
		LBSpec:                 lbspec,
		Ingress:                ing,
		Services:               services,
		RoutingPolicies:        routePolicies,
		RuleSets:               ruleSets,
		HostnameDetails:        hostnameDetailsCollection,
		Certificates:           certificateCollection,
		_serviceAndNodeMapping: serviceAndNodeMapping,
	}
	if err := setupBackendSetsForSpec(spec, ing, logger); err != nil {
		return nil, err
	}

	return spec, nil
}

var discoveredSubnet = ""

func tryFindLoadbalancerSubnet(ociClient ociclient.Interface, logger *zap.Logger) (string, error) {
	if discoveredSubnet != "" {
		return discoveredSubnet, nil
	}
	meta, err := metadata.New().Get()
	if err != nil {
		return "", err
	}
	ctx := context.Background()
	instanceVNIC, err := ociClient.Compute().GetPrimaryVNICForInstance(ctx, meta.CompartmentID, meta.ID)
	if err != nil {
		return "", err
	}
	logger.Debug("Instance VNIC: " + *instanceVNIC.Id)
	instanceSubnetId := *instanceVNIC.SubnetId
	instanceSubnet, err := ociClient.Networking().GetSubnet(ctx, *instanceVNIC.SubnetId)
	logger.Debug("Instance Subnet: " + instanceSubnetId + " VCN: " + *instanceSubnet.VcnId)
	if err != nil {
		return "", err
	}
	subnets, err := ociClient.Networking().ListSubnets(ctx, meta.CompartmentID, *instanceSubnet.VcnId)
	for _, s := range subnets {
		if !*s.ProhibitPublicIpOnVnic {
			logger.Sugar().Infof("Found public subnet %s in VCN %s. Choosing it as Loadbalancer subnet", *instanceSubnet.VcnId, *s.Id)
			return *s.Id, nil
		}
	}
	if err != nil {
		return "", err
	}
	logger.Sugar().Warnf("No public subnets found in VCN %s. Choosing instance subnet %s as loadbalancer subnet", *instanceSubnet.VcnId, instanceSubnetId)
	return instanceSubnetId, nil
}

func getLoadBalancerSubnetIds(config configholder.ConfigHolder, ing *networking.Ingress, ociClient ociclient.Interface, logger *zap.Logger) (subnetIds []string, err error) {
	if subnet1 := GetAnnotation(ing, AnnotationLoadBalancerSubnet1); subnet1 != "" {
		subnetIds = append(subnetIds, subnet1)
	}
	if subnet2 := GetAnnotation(ing, AnnotationLoadBalancerSubnet2); subnet2 != "" {
		subnetIds = append(subnetIds, subnet2)
	}
	if subnetIds != nil {
		return subnetIds, nil
	}
	subnetIds = config.GetSubnetIds()
	if subnetIds != nil {
		return subnetIds, nil
	}

	logger.Warn("No default loadbalancer subnet is configured. Try to find one from instance VCN")
	subnetId, err := tryFindLoadbalancerSubnet(ociClient, logger)
	if err != nil {
		return nil, errors.Wrap(err, "Could not get subnetIds. Error while trying to discover SubnetIds")
	}
	if subnetId != "" {
		subnetIds = append(subnetIds, subnetId)
		return subnetIds, nil
	}
	return nil, errors.New("Could not get subnetIds")
}

func setupBackendSetsForSpec(spec *IngressLBSpec, ing *networking.Ingress, logger *zap.Logger) error {
	backendSetDetails := map[string]loadbalancer.BackendSetDetails{}

	loadbalancerPolicy, err := getLoadBalancerPolicy(ing)
	if err != nil {
		return err
	}

	var sslConfig *oci.SSLConfig = nil // TODO

	for _, svc := range spec.Services {
		backendSetList, err := oci.GetBackendSets(logger.Sugar(), svc, spec.NodesForService(svc.Name), sslConfig, loadbalancerPolicy)
		if err != nil {
			return err
		}
		for name, backendset := range backendSetList {
			backendSetDetails[name] = backendset
		}
	}

	backendSetDetails[DummyBackendSetName] = loadbalancer.BackendSetDetails{ // TODO
		Policy:   utils.PtrToString(loadbalancerPolicy),
		Backends: []loadbalancer.BackendDetails{},
		HealthChecker: &loadbalancer.HealthCheckerDetails{
			Protocol: utils.PtrToString("HTTP"),
			// Do not change following fields,as it could cause unwanted change detection during next reconciliation.
			UrlPath:          utils.PtrToString("/"),
			Port:             utils.PtrToInt(0),
			IntervalInMillis: utils.PtrToInt(10000),
			TimeoutInMillis:  utils.PtrToInt(3000),
			Retries:          utils.PtrToInt(3),
		},
	}
	spec.BackendSets = backendSetDetails
	return nil
}
