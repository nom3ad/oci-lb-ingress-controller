package ingress

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/nom3ad/oci-lb-ingress-controller/src/utils"
	"github.com/oracle/oci-go-sdk/v46/loadbalancer"
	"github.com/pkg/errors"
	networking "k8s.io/api/networking/v1"
)

func createDeafultBackendRoutingRule(backendSetName string) (*loadbalancer.RoutingRule, error) {
	ruleName := "default-backend-route"
	condition := fmt.Sprintf("all(http.request.url.path sw '%s')", "/")
	return &loadbalancer.RoutingRule{
		Name:      &ruleName,
		Condition: &condition,
		Actions: []loadbalancer.Action{
			loadbalancer.ForwardToBackendSet{
				BackendSetName: &backendSetName,
			},
		},
	}, nil
}

func createRoutingRule(ingresPath networking.HTTPIngressPath, backendSetName string, host string) (*loadbalancer.RoutingRule, error) {
	ruleName := utils.ObjectHash(ingresPath, 22) // max 32 , ^[a-zA-Z_][a-zA-Z_0-9]*$
	path := ingresPath.Path
	// https://docs.oracle.com/en-us/iaas/Content/Balance/Concepts/routing_policy_conditions.htm
	var conditions []string
	if host != "" {
		if hostnameCondition := createHostnameCondition(host); hostnameCondition != "" {
			conditions = append(conditions, hostnameCondition)
		}
	}
	switch *ingresPath.PathType {
	case networking.PathTypeExact:
		exactPathMatchCondition := fmt.Sprintf("http.request.url.path eq '%s'", path)
		conditions = append(conditions, exactPathMatchCondition)
	case networking.PathTypePrefix:
		prefixPathMatchCondition := fmt.Sprintf("http.request.url.path sw '%s'", path)
		conditions = append(conditions, prefixPathMatchCondition)
	case networking.PathTypeImplementationSpecific:
		customCondition, err := processImplementationSpecificPath(path)
		if err != nil {
			return nil, err
		}
		// https://docs.oracle.com/en-us/iaas/Content/Balance/Concepts/routing_policy_conditions.htm#routing_policy_conditions_conditions
		if regexp.MustCompile(`^(not )?(any|all)\(`).MatchString(customCondition) {
			// customCondition is already using multiple predicates. So use as
			// TODO: inject hostnameCondition to customCondition by proper parsing
			if matches := regexp.MustCompile(`^all\((.*)\)$`).FindStringSubmatch(customCondition); len(matches) > 1 {
				conditions = append(conditions, strings.Split(matches[1], ",")...)
			} else {
				conditions = []string{customCondition}
			}
		} else {
			conditions = append(conditions, customCondition)
		}
	default:
		return nil, errors.Errorf("Unknown ingress path type: %s", *ingresPath.PathType)
	}

	var condition string
	if len(conditions) == 1 {
		condition = conditions[0]
	} else {
		condition = fmt.Sprintf("all(%s)", strings.Join(conditions, ","))
	}
	return &loadbalancer.RoutingRule{
		Name:      &ruleName,
		Condition: &condition,
		Actions: []loadbalancer.Action{
			loadbalancer.ForwardToBackendSet{
				BackendSetName: &backendSetName,
			},
		},
	}, nil
}

func createHostnameCondition(host string) string {
	// Form Kubernetes documentation:
	// .........
	// Host is the fully qualified domain name of a network host, as defined by RFC 3986.
	// Note the following deviations from the "host" part of the URI as defined in RFC 3986:
	// 1. IPs are not allowed. Currently an IngressRuleValue can only apply to the IP in the Spec of the parent Ingress.
	// 2. The `:` delimiter is not respected because ports are not allowed.
	// Currently the port of an Ingress is implicitly :80 for http and :443 for https.
	//
	// Both these may change in the future. Incoming requests are matched against the host before the IngressRuleValue.
	// If the host is unspecified, the Ingress routes all traffic based on the specified IngressRuleValue.
	// Host can be "precise" which is a domain name without the terminating dot of a network host (e.g. "foo.bar.com") or "wildcard",
	//  which is a domain name prefixed with a single wildcard label (e.g. "*.foo.com").
	// The wildcard character '*' must appear by itself as the first DNS label and matches only a single label.
	// You cannot have a wildcard label by itself (e.g. Host == "*").
	// Requests will be matched against the Host field in the following way:
	//   1. If Host is precise, the request matches this rule if the http host header is equal to Host.
	//   2. If Host is a wildcard, then the request matches this rule if the http host header is to equal to the suffix (removing the first label) of the wildcard rule.
	// .........

	if strings.HasPrefix(host, "*.") {
		// return fmt.Sprintf("http.request.headers[(i 'Host')] ew (i '%s')", host[2:])
		//XXX: OCI LB does not support ew/sw matchers on Map Type values. Gets an err: invalid operand types for matcher IREndsWithMatcher, left: IRStringListValue, right: IRStringValue
		// But it would be okay to not have Host header matching rule, because we have listener configured with correct wildcard hostname
		return ""
	}
	return fmt.Sprintf("http.request.headers[(i 'Host')] eq (i '%s')", host)
}

func processImplementationSpecificPath(pathValue string) (string, error) {
	// https://docs.oracle.com/en-us/iaas/Content/Balance/Concepts/routing_policy_conditions.htm
	if pathValue == "" {
		return "", errors.Errorf("Invalid path %q", pathValue)
	}
	routingPolicyRuleConditionPrefix := "condition:"
	if strings.HasPrefix(pathValue, routingPolicyRuleConditionPrefix) && len(strings.TrimSpace(pathValue)) > len(routingPolicyRuleConditionPrefix) {
		return strings.TrimSpace(strings.TrimPrefix(pathValue, "condition:")), nil
	}
	urlParsed, err := url.Parse(pathValue)
	if err != nil {
		return "", errors.Wrapf(err, "Invalid path %q", pathValue)
	}
	if urlParsed.Fragment != "" || urlParsed.Scheme != "" || urlParsed.Host != "" || urlParsed.RawQuery != "" {
		return "", errors.Errorf("Invalid path %q", pathValue)
	}
	// TODO: support negation rules
	path := urlParsed.Path
	if !strings.Contains(path, "*") { // No wild card
		return fmt.Sprintf("all(http.request.url.path eq '%s')", path), nil
	}
	if matches, _ := regexp.MatchString("^[^\\*]+\\*[^\\*]+$", path); matches { // single wildcard in middle. eg: /api/*/example
		parts := strings.Split(path, "*")
		return fmt.Sprintf("all(http.request.url.path sw '%s', http.request.url.path ew '%s')", parts[0], parts[1]), nil
	}
	if matches, _ := regexp.MatchString("^\\*[^\\*]+$", path); matches { // starts with wildcard. eg: */example
		return fmt.Sprintf("all(http.request.url.path ew '%s')", path[1:]), nil
	}
	if matches, _ := regexp.MatchString("^[^\\*]+\\*$", path); matches { // ends with wildcard. eg: /example/*
		return fmt.Sprintf("all(http.request.url.path sw '%s')", path[:len(path)-1]), nil
	}
	if matches, _ := regexp.MatchString("^\\*+[^\\*]+\\*+$", path); matches { // starts and ends with wildcard. eg: */example/*
		return fmt.Sprintf("all(http.request.url.path cw '%s')", path[1:len(path)-1]), nil
	}
	return "", errors.Errorf("Invalid path %q", pathValue)
}
