package ingress

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/nom3ad/oci-lb-ingress-controller/src/utils"
	"github.com/oracle/oci-go-sdk/v46/loadbalancer"
	networking "k8s.io/api/networking/v1"
)

const DummyBackendSetName = "dummy"

func createListenerDetails(ing *networking.Ingress, hostnameDetails *loadbalancer.HostnameDetails, sslConfigDetails *loadbalancer.SslConfigurationDetails) (string, loadbalancer.ListenerDetails) {
	var protocol string
	var port int
	if sslConfigDetails != nil {
		protocol = "HTTP2" //TODO: Or retain HTTP, if an annotation enforces it
		if sslConfigDetails.CipherSuiteName == nil {
			// As of now, HTTP2 listener can only support a default cipher suite 'oci-default-http2-ssl-cipher-suite-v1'
			sslConfigDetails.CipherSuiteName = utils.PtrToString("oci-default-http2-ssl-cipher-suite-v1")
		}
		port = 443
	} else {
		protocol = "HTTP"
		port = 80
	}
	var hostname string
	var hostnameNames []string
	if hostnameDetails != nil {
		hostname = *hostnameDetails.Hostname
		hostnameNames = []string{*hostnameDetails.Name}
	}
	listener := loadbalancer.ListenerDetails{
		// .DefaultBackendSetName must not be null
		DefaultBackendSetName: utils.PtrToString(DummyBackendSetName),
		Protocol:              &protocol,
		Port:                  &port,
		HostnameNames:         hostnameNames,
		SslConfiguration:      sslConfigDetails,
		// RoutingPolicyName: // Can't set now. Will set after after defining `RoutingPolicy` struct // TODO: Accept via arg
		// ConnectionConfiguration: ,
		// RuleSetNames: ,

	}

	name := GetListenerName(hostname)
	return name, listener
}

func createListenerDetailsAndRulesetDetailsForHTTPSRedirect(hostnameNames []string) (rulesetName string, ruleSet loadbalancer.RuleSetDetails, listenerName string, listener loadbalancer.ListenerDetails) {
	rulesetName = "https_301_redirection" // ^[a-zA-Z_][a-zA-Z_0-9]*$
	ruleSet = loadbalancer.RuleSetDetails{
		Items: []loadbalancer.Rule{
			loadbalancer.RedirectRule{
				Conditions: []loadbalancer.RuleCondition{
					loadbalancer.PathMatchCondition{
						AttributeValue: utils.PtrToString("/"),
						Operator:       loadbalancer.PathMatchConditionOperatorPrefixMatch,
					},
				},
				RedirectUri: &loadbalancer.RedirectUri{
					Protocol: utils.PtrToString("https"),
					Port:     utils.PtrToInt(443),
					Host:     utils.PtrToString("{host}"),
					Path:     utils.PtrToString("/{path}"),
					Query:    utils.PtrToString("?{query}"),
				},
				ResponseCode: utils.PtrToInt(301),
			},
		},
	}
	listenerName = "http-to-https-redirector"
	listener = loadbalancer.ListenerDetails{
		// .DefaultBackendSetName must not be null
		DefaultBackendSetName: utils.PtrToString(DummyBackendSetName),
		Protocol:              utils.PtrToString("HTTP"),
		Port:                  utils.PtrToInt(80),
		HostnameNames:         hostnameNames,
		RuleSetNames:          []string{rulesetName},
	}
	return rulesetName, ruleSet, listenerName, listener
}

func createDefaultBackendListenerDetails(targetBackendSetName string) (listenerName string, listener loadbalancer.ListenerDetails) {
	listenerName = "DefaultBackend-http"
	listener = loadbalancer.ListenerDetails{
		// .defaultBackendSetName must not be null
		DefaultBackendSetName: utils.PtrToString(targetBackendSetName),
		Protocol:              utils.PtrToString("HTTP"),
		Port:                  utils.PtrToInt(80),
		HostnameNames:         nil,
	}
	return listenerName, listener
}

// createSansVirtualHostListenerDetails creates ListenerDetails for default listener
// that will handle requests that does not match Host value
func createSansVirtualHostListenerDetails() (listenerName string, listener loadbalancer.ListenerDetails) {
	listenerName = "Sans-VirtualHost-HTTP"
	listener = loadbalancer.ListenerDetails{
		// .defaultBackendSetName must not be null
		DefaultBackendSetName: utils.PtrToString(DummyBackendSetName),
		Protocol:              utils.PtrToString("HTTP"),
		Port:                  utils.PtrToInt(80),
		HostnameNames:         nil,
	}
	return listenerName, listener
}

const lbNamePrefixEnvVar = "LOAD_BALANCER_PREFIX"

// GetLoadBalancerName gets the name of the load balancer based on the Ingress
func GetLoadBalancerName(namespace string, ingressName string) string {
	// namespace, ingressName will be valid DNS label (63 chars max)
	prefix := os.Getenv(lbNamePrefixEnvVar)
	if prefix != "" && !strings.HasSuffix(prefix, "_") {
		// Add the trailing hyphen if it's missing
		prefix += "_"
	}
	name := fmt.Sprintf("%s%s_%s", prefix, namespace, ingressName)

	if len(name) > 1024 {
		// 1024 is the max length for display name
		// https://docs.us-phoenix-1.oraclecloud.com/api/#/en/loadbalancer/20170115/requests/UpdateLoadBalancerDetails
		name = name[:1024]
	}

	return name
}

func getRoutingPolicyName(hostname string) string {
	// name must match "^[a-zA-Z_][a-zA-Z_0-9]{0,31}$"; name size must be between 1 and 32
	name := strings.ToLower(hostname)
	name = strings.ReplaceAll(name, "*.", "S_")
	name = strings.ReplaceAll(name, ".", "_")
	name = strings.ReplaceAll(name, "-", "_")
	name = strings.ReplaceAll(name, "__", "_")

	// if hostname starts with number, adjust name
	if name != "" && name[0] >= '0' && name[0] <= '9' {
		name = "X_" + name
	}
	// if hostname is empty, routing policy name starts with '_'
	name = utils.SafeSlice(utils.SafeSlice(name, 0, 25)+"_"+utils.ByteAlphaNumericDigest([]byte(hostname), 32), 0, 32)
	if matched, err := regexp.Match("^[a-zA-Z_][a-zA-Z_0-9]{1,31}$", []byte(name)); !matched || err != nil {
		panic("Invalid Routing policy name.")
	}
	return name
}

func GetListenerName(hostname string) string {
	if hostname == "" {
		return "any-host"
	}
	name := strings.ToLower(hostname)
	name = strings.ReplaceAll(name, "*.", "STAR")
	name = strings.ReplaceAll(name, ".", "DOT")
	if len(name) > 240 {
		// max length is 255
		name = utils.SafeSlice(name[:240]+utils.ByteAlphaNumericDigest([]byte(hostname), 15), 0, 255)
	}
	if matched, err := regexp.Match("^[a-zA-Z0-9_-]{1,255}$", []byte(name)); !matched || err != nil {
		panic("Invalid Listener name.")
	}
	return name
}

func getHostnameName(hostname string) string {
	name := hostname
	if len(name) > 240 {
		// max length is 255
		name = utils.SafeSlice(name[:240]+utils.ByteAlphaNumericDigest([]byte(hostname), 15), 0, 255)
	}
	return name
}
