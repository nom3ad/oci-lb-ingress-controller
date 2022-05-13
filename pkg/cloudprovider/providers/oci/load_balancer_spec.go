package oci

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/nom3ad/oci-lb-ingress-controller/src/helpers"
	"github.com/oracle/oci-go-sdk/v46/common"
	"github.com/oracle/oci-go-sdk/v46/loadbalancer"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	networking "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/util/sets"
)

// certificateData is a structure containing the data about a K8S secret required
// to store SSL information required for BackendSets and Listeners
type certificateData struct {
	Name       string
	CACert     []byte
	PublicCert []byte
	PrivateKey []byte
	Passphrase []byte
}

type sslSecretReader interface {
	readSSLSecret(ns, name string) (sslSecret *certificateData, err error)
}

type noopSSLSecretReader struct{}

func (ssr noopSSLSecretReader) readSSLSecret(ns, name string) (sslSecret *certificateData, err error) {
	return nil, nil
}

// SSLConfig is a description of a SSL certificate.
type SSLConfig struct {
	Ports sets.Int

	ListenerSSLSecretName      string
	ListenerSSLSecretNamespace string

	BackendSetSSLSecretName      string
	BackendSetSSLSecretNamespace string

	sslSecretReader
}

// func requiresCertificate(svc *corev1.Service) bool {
// 	_, ok :=  GetAnnotation(svc, AnnotationLoadBalancerSSLPorts)
// 	return ok
// }

// NewSSLConfig constructs a new SSLConfig.
func NewSSLConfig(secretListenerString string, secretBackendSetString string, service *corev1.Service, ports []int, ssr sslSecretReader) *SSLConfig {
	if ssr == nil {
		ssr = noopSSLSecretReader{}
	}

	listenerSecretName, listenerSecretNamespace := getSecretParts(secretListenerString, service)
	backendSecretName, backendSecretNamespace := getSecretParts(secretBackendSetString, service)

	return &SSLConfig{
		Ports:                        sets.NewInt(ports...),
		ListenerSSLSecretName:        listenerSecretName,
		ListenerSSLSecretNamespace:   listenerSecretNamespace,
		BackendSetSSLSecretName:      backendSecretName,
		BackendSetSSLSecretNamespace: backendSecretNamespace,
		sslSecretReader:              ssr,
	}
}

// LBSpec holds the data required to build a OCI load balancer from a
// kubernetes service.
type LBSpec struct {
	Name           string
	Shape          string
	FlexMin        *int
	FlexMax        *int
	Subnets        []string
	Internal       bool
	Listeners      map[string]loadbalancer.ListenerDetails
	BackendSets    map[string]loadbalancer.BackendSetDetails
	LoadBalancerIP string

	Ports       map[string]PortSpec
	SourceCIDRs []string   // TODO
	SSLConfig   *SSLConfig // TODO

	//
	SecurityListManager     securityListManager
	NetworkSecurityGroupIds []string
	Nodes                   []*corev1.Node
}

// Certificates builds a map of required SSL certificates.
func (s *LBSpec) Certificates() (map[string]loadbalancer.CertificateDetails, error) {
	certs := make(map[string]loadbalancer.CertificateDetails)

	if s.SSLConfig == nil {
		return certs, nil
	}

	if s.SSLConfig.ListenerSSLSecretName != "" {
		cert, err := s.SSLConfig.readSSLSecret(s.SSLConfig.ListenerSSLSecretNamespace, s.SSLConfig.ListenerSSLSecretName)
		if err != nil {
			return nil, errors.Wrap(err, "reading SSL Listener Secret")
		}
		certs[s.SSLConfig.ListenerSSLSecretName] = loadbalancer.CertificateDetails{
			CertificateName:   &s.SSLConfig.ListenerSSLSecretName,
			CaCertificate:     common.String(string(cert.CACert)),
			PublicCertificate: common.String(string(cert.PublicCert)),
			PrivateKey:        common.String(string(cert.PrivateKey)),
			Passphrase:        common.String(string(cert.Passphrase)),
		}
	}

	if s.SSLConfig.BackendSetSSLSecretName != "" {
		cert, err := s.SSLConfig.readSSLSecret(s.SSLConfig.BackendSetSSLSecretNamespace, s.SSLConfig.BackendSetSSLSecretName)
		if err != nil {
			return nil, errors.Wrap(err, "reading SSL Backend Secret")
		}
		certs[s.SSLConfig.BackendSetSSLSecretName] = loadbalancer.CertificateDetails{
			CertificateName:   &s.SSLConfig.BackendSetSSLSecretName,
			CaCertificate:     common.String(string(cert.CACert)),
			PublicCertificate: common.String(string(cert.PublicCert)),
			PrivateKey:        common.String(string(cert.PrivateKey)),
			Passphrase:        common.String(string(cert.Passphrase)),
		}
	}
	return certs, nil
}

// func getLoadBalancerSourceRanges(service *corev1.Service) ([]string, error) {
// 	sourceRanges, err := helpers.GetLoadBalancerSourceRanges(service)
// 	if err != nil {
// 		return []string{}, err
// 	}

// 	sourceCIDRs := make([]string, 0, len(sourceRanges))
// 	for _, sourceRange := range sourceRanges {
// 		sourceCIDRs = append(sourceCIDRs, sourceRange.String())
// 	}

// 	return sourceCIDRs, nil
// }

func getPortSpecName(protocol string, port int) string {
	return fmt.Sprintf("%s-%d", protocol, port)
}

func getPortForIngress(ing *networking.Ingress) (map[string]PortSpec, error) {
	// An Ingress does not expose arbitrary ports or protocols
	ports := make(map[string]PortSpec)
	ports["HTTP"] = PortSpec{
		// BackendPort:       int(servicePort.NodePort),
		ListenerPort: 80,
		// HealthCheckerPort: *healthChecker.Port,
	}
	return ports, nil
}

// func getPorts(svc *corev1.Service) (map[string]PortSpec, error) {
// 	ports := make(map[string]PortSpec)
// 	for _, servicePort := range svc.Spec.Ports {
// 		name := getPortSpecName(string(servicePort.Protocol), int(servicePort.Port))
// 		healthChecker, err := getHealthChecker(svc)
// 		if err != nil {
// 			return nil, err
// 		}
// 		ports[name] = PortSpec{
// 			BackendPort:       int(servicePort.NodePort),
// 			ListenerPort:      int(servicePort.Port),
// 			HealthCheckerPort: *healthChecker.Port,
// 		}
// 	}
// 	return ports, nil
// }

func getBackends(logger *zap.SugaredLogger, nodes []*corev1.Node, nodePort int32) []loadbalancer.BackendDetails {
	backends := make([]loadbalancer.BackendDetails, 0)
	for _, node := range nodes {
		nodeAddressString := common.String(helpers.NodeInternalIP(node))
		if *nodeAddressString == "" {
			logger.Warnf("Node %q has an empty Internal IP address.", node.Name)
			continue
		}
		backends = append(backends, loadbalancer.BackendDetails{
			IpAddress: nodeAddressString,
			Port:      common.Int(int(nodePort)),
			Weight:    common.Int(1),
		})
	}
	return backends
}

func GetBackendSets(logger *zap.SugaredLogger, svc *corev1.Service, nodes []*corev1.Node, sslCfg *SSLConfig, loadbalancerPolicy string) (map[string]loadbalancer.BackendSetDetails, error) {
	backendSets := make(map[string]loadbalancer.BackendSetDetails)
	for _, servicePort := range svc.Spec.Ports {
		name := GetBackendSetName(svc.Name, string(servicePort.Protocol), int(servicePort.Port))
		port := int(servicePort.Port)
		var secretName string
		if sslCfg != nil && len(sslCfg.BackendSetSSLSecretName) != 0 {
			secretName = sslCfg.BackendSetSSLSecretName
		}
		healthChecker, err := getHealthChecker(svc)
		if err != nil {
			return nil, err
		}
		backendSets[name] = loadbalancer.BackendSetDetails{
			Policy:           common.String(loadbalancerPolicy),
			Backends:         getBackends(logger, nodes, servicePort.NodePort),
			HealthChecker:    healthChecker,
			SslConfiguration: getSSLConfiguration(sslCfg, secretName, port),
		}
	}
	return backendSets, nil
}

func getHealthChecker(svc *corev1.Service) (*loadbalancer.HealthCheckerDetails, error) {
	// Setting default values as per defined in the doc (https://docs.cloud.oracle.com/en-us/iaas/Content/Balance/Tasks/editinghealthcheck.htm#console)
	var retries = 3
	if r := GetAnnotation(svc, AnnotationLoadBalancerHealthCheckRetries); r != "" {
		rInt, err := strconv.Atoi(r)
		if err != nil {
			return nil, fmt.Errorf("invalid value: %s provided for annotation: %s", r, AnnotationLoadBalancerHealthCheckRetries)
		}
		retries = rInt
	}
	// Setting default values as per defined in the doc (https://docs.cloud.oracle.com/en-us/iaas/Content/Balance/Tasks/editinghealthcheck.htm#console)
	var intervalInMillis = 10000
	if i := GetAnnotation(svc, AnnotationLoadBalancerHealthCheckInterval); i != "" {
		iInt, err := strconv.Atoi(i)
		if err != nil {
			return nil, fmt.Errorf("invalid value: %s provided for annotation: %s", i, AnnotationLoadBalancerHealthCheckInterval)
		}
		intervalInMillis = iInt
	}
	// Setting default values as per defined in the doc (https://docs.cloud.oracle.com/en-us/iaas/Content/Balance/Tasks/editinghealthcheck.htm#console)
	var timeoutInMillis = 3000
	if t := GetAnnotation(svc, AnnotationLoadBalancerHealthCheckTimeout); t != "" {
		tInt, err := strconv.Atoi(t)
		if err != nil {
			return nil, fmt.Errorf("invalid value: %s provided for annotation: %s", t, AnnotationLoadBalancerHealthCheckTimeout)
		}
		timeoutInMillis = tInt
	}
	checkPath, checkPort := helpers.GetServiceHealthCheckPathPort(svc)
	if checkPath != "" {
		return &loadbalancer.HealthCheckerDetails{
			Protocol:         common.String(lbNodesHealthCheckProto),
			UrlPath:          &checkPath,
			Port:             common.Int(int(checkPort)),
			Retries:          &retries,
			IntervalInMillis: &intervalInMillis,
			TimeoutInMillis:  &timeoutInMillis,
		}, nil
	}

	return &loadbalancer.HealthCheckerDetails{
		Protocol:         common.String(lbNodesHealthCheckProto),
		UrlPath:          common.String(lbNodesHealthCheckPath),
		Port:             common.Int(lbNodesHealthCheckPort),
		Retries:          &retries,
		IntervalInMillis: &intervalInMillis,
		TimeoutInMillis:  &timeoutInMillis,
	}, nil
}

func getSSLConfiguration(cfg *SSLConfig, name string, port int) *loadbalancer.SslConfigurationDetails {
	if cfg == nil || !cfg.Ports.Has(port) || len(name) == 0 {
		return nil
	}
	return &loadbalancer.SslConfigurationDetails{
		CertificateName:       &name,
		VerifyDepth:           common.Int(0),
		VerifyPeerCertificate: common.Bool(false),
	}
}

//! Not In Use
func getListeners(svc *corev1.Service, sslCfg *SSLConfig) (map[string]loadbalancer.ListenerDetails, error) {
	// Determine if connection idle timeout has been specified
	var connectionIdleTimeout *int64
	connectionIdleTimeoutAnnotation := GetAnnotation(svc, AnnotationLoadBalancerConnectionIdleTimeout)
	if connectionIdleTimeoutAnnotation != "" {
		timeout, err := strconv.ParseInt(connectionIdleTimeoutAnnotation, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("error parsing service annotation: %s=%s",
				AnnotationLoadBalancerConnectionIdleTimeout,
				connectionIdleTimeoutAnnotation,
			)
		}

		connectionIdleTimeout = common.Int64(timeout)
	}

	// Determine if proxy protocol has been specified
	var proxyProtocolVersion *int
	proxyProtocolVersionAnnotation := GetAnnotation(svc, ConnectionProxyProtocolVersion)
	if proxyProtocolVersionAnnotation != "" {
		version, err := strconv.Atoi(proxyProtocolVersionAnnotation)
		if err != nil {
			return nil, fmt.Errorf("error parsing service annotation: %s=%s",
				ConnectionProxyProtocolVersion,
				proxyProtocolVersionAnnotation,
			)
		}

		proxyProtocolVersion = common.Int(version)
	}

	listeners := make(map[string]loadbalancer.ListenerDetails)
	for _, servicePort := range svc.Spec.Ports {
		protocol := string(servicePort.Protocol)
		// Annotation overrides the protocol.
		if p := GetAnnotation(svc, AnnotationLoadBalancerBEProtocol); p != "" {
			// Default
			if p == "" {
				p = "TODO:DefaultLoadBalancerBEProtocol"
			}
			if strings.EqualFold(p, "HTTP") || strings.EqualFold(p, "TCP") {
				protocol = p
			} else {
				return nil, fmt.Errorf("invalid backend protocol %q requested for load balancer listener. Only 'HTTP' and 'TCP' protocols supported", p)
			}
		}
		port := int(servicePort.Port)
		var secretName string
		if sslCfg != nil && len(sslCfg.ListenerSSLSecretName) != 0 {
			secretName = sslCfg.ListenerSSLSecretName
		}
		sslConfiguration := getSSLConfiguration(sslCfg, secretName, port)
		name := getListenerName(protocol, port)

		listener := loadbalancer.ListenerDetails{
			DefaultBackendSetName: common.String(GetBackendSetName(svc.Name, string(servicePort.Protocol), int(servicePort.Port))),
			Protocol:              &protocol,
			Port:                  &port,
			SslConfiguration:      sslConfiguration,
		}

		// If proxy protocol has been set, we also need to set connectionIdleTimeout
		// because it's a required parameter as per the LB API contract.
		// The default value is dependent on the protocol used for the listener.
		actualConnectionIdleTimeout := connectionIdleTimeout
		if proxyProtocolVersion != nil && connectionIdleTimeout == nil {
			// At that point LB only supports HTTP and TCP
			defaultIdleTimeoutPerProtocol := map[string]int64{
				"HTTP": lbConnectionIdleTimeoutHTTP,
				"TCP":  lbConnectionIdleTimeoutTCP,
			}
			actualConnectionIdleTimeout = common.Int64(defaultIdleTimeoutPerProtocol[strings.ToUpper(protocol)])
		}

		if actualConnectionIdleTimeout != nil {
			listener.ConnectionConfiguration = &loadbalancer.ConnectionConfiguration{
				IdleTimeout:                    actualConnectionIdleTimeout,
				BackendTcpProxyProtocolVersion: proxyProtocolVersion,
			}
		}

		listeners[name] = listener
	}

	return listeners, nil
}

func getSecretParts(secretString string, service *corev1.Service) (name string, namespace string) {
	if secretString == "" {
		return "", ""
	}
	if !strings.Contains(secretString, "/") {
		return secretString, service.Namespace
	}
	parts := strings.Split(secretString, "/")
	return parts[1], parts[0]
}

func getNetworkSecurityGroupIds(svc *corev1.Service) ([]string, error) {
	var nsgList []string
	networkSecurityGroupIds := GetAnnotation(svc, AnnotationLoadBalancerNetworkSecurityGroups)
	if networkSecurityGroupIds == "" {
		return nsgList, nil
	}

	numOfNsgIds := 0
	for _, nsgOCID := range helpers.RemoveDuplicatesFromList(strings.Split(strings.ReplaceAll(networkSecurityGroupIds, " ", ""), ",")) {
		numOfNsgIds++
		if numOfNsgIds > lbMaximumNetworkSecurityGroupIds {
			return nil, fmt.Errorf("invalid number of Network Security Groups (Max: 5) provided for annotation: %s", AnnotationLoadBalancerNetworkSecurityGroups)
		}
		if nsgOCID != "" {
			nsgList = append(nsgList, nsgOCID)
			continue
		}
		return nil, fmt.Errorf("invalid NetworkSecurityGroups OCID: [%s] provided for annotation: %s", networkSecurityGroupIds, AnnotationLoadBalancerNetworkSecurityGroups)
	}

	return nsgList, nil
}

func IsInternalLB(ingOrSvc interface{}) (bool, error) {
	var obj AnnotatedObject
	switch o := ingOrSvc.(type) {
	case *corev1.Service:
		panic("TODO")
	case *networking.Ingress:
		obj = o
	default:
		panic("shouldn't be here!")
	}

	if private := GetAnnotation(obj, AnnotationLoadBalancerInternal); private != "" {
		internal, err := strconv.ParseBool(private)
		if err != nil {
			return false, errors.Wrap(err, fmt.Sprintf("invalid value: %s provided for annotation: %s", private, AnnotationLoadBalancerInternal))
		}
		return internal, nil
	}
	return false, nil
}

func GetLoadBalancerIP(ingOrSvc interface{}) (ipAddress string, err error) {
	switch obj := ingOrSvc.(type) {
	case *corev1.Service:
		ipAddress = obj.Spec.LoadBalancerIP
		panic("TODO")
	case *networking.Ingress:
		ipAddress = GetAnnotation(obj, AnnotationLoadBalancerReservedIP)
	default:
		panic("shouldn't be here!")
	}
	if ipAddress == "" {
		return "", nil
	}

	//checks the validity of loadbalancerIP format
	if net.ParseIP(ipAddress) == nil {
		return "", fmt.Errorf("invalid value %q provided for LoadBalancerIP", ipAddress)
	}

	//checks if private loadbalancer is trying to use reservedIP
	isInternal, err := IsInternalLB(ingOrSvc)
	if isInternal {
		return "", fmt.Errorf("invalid service: cannot create a private load balancer with Reserved IP")
	}
	return ipAddress, err
}
