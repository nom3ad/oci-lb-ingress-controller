package oci

import "strings"

type AnnotatedObject interface {
	GetAnnotations() map[string]string
}

const IngressAnnotationPrefix = "ingress.beta.kubernetes.io/"
const ServiceAnnotationPrefix = "service.beta.kubernetes.io/"

func GetAnnotation(obj AnnotatedObject, name string) string {
	return obj.GetAnnotations()[IngressAnnotationPrefix+name]
}

func GetAnnotationWithLowercase(obj AnnotatedObject, name string) string {
	return strings.ToLower(GetAnnotation(obj, name))
}

// https://github.dev/oracle/oci-cloud-controller-manager
const (

	// OCI Ingress Annotations

	// LoadBalancerInternal is an annotation for
	// specifying that a load balancer should be internal.
	AnnotationLoadBalancerInternal = "oci-load-balancer-internal"

	// AnnotationLoadBalancerShape is an annotation for
	// specifying the Shape of a load balancer. The shape is a template that
	// determines the load balancer's total pre-provisioned maximum capacity
	// (bandwidth) for ingress plus egress traffic. Available shapes include
	// "100Mbps", "400Mbps", "8000Mbps", and "flexible". When using
	// "flexible" ,it is required to also supply
	AnnotationLoadBalancerShape = "oci-load-balancer-shape"

	// AnnotationLoadBalancerShapeFlexMin is an annotation for
	// specifying the minimum bandwidth in Mbps if the LB shape is flex.
	AnnotationLoadBalancerShapeFlexMin = "oci-load-balancer-shape-flex-min"

	// AnnotationLoadBalancerShapeFlexMax is an annotation for
	// specifying the maximum bandwidth in Mbps if the shape is flex.
	AnnotationLoadBalancerShapeFlexMax = "oci-load-balancer-shape-flex-max"

	// AnnotationLoadBalancerSubnet1 is an annotation for
	// specifying the first subnet of a load balancer.
	AnnotationLoadBalancerSubnet1 = "oci-load-balancer-subnet1"

	// AnnotationLoadBalancerSubnet2 is an annotation for
	// specifying the second subnet of a load balancer.
	AnnotationLoadBalancerSubnet2 = "oci-load-balancer-subnet2"

	// AnnotationLoadBalancerConnectionIdleTimeout is the annotation used
	// on the Annotationloadbalancer to specify the idle connection timeout.
	AnnotationLoadBalancerConnectionIdleTimeout = "oci-load-balancer-connection-idle-timeout"

	// AnnotationLoadBalancerHealthCheckRetries is the annotation used
	// on the Annotationloadbalancer to specify the number of retries to attempt before a backend server is considered "unhealthy".
	AnnotationLoadBalancerHealthCheckRetries = "oci-load-balancer-health-check-retries"

	// AnnotationLoadBalancerHealthCheckInterval is an annotation for
	// specifying the interval between health checks, in milliseconds.
	AnnotationLoadBalancerHealthCheckInterval = "oci-load-balancer-health-check-interval"

	// AnnotationLoadBalancerHealthCheckTimeout is an annotation for
	// specifying the maximum time, in milliseconds, to wait for a reply to a health check. A health check is successful only if a reply
	// returns within this timeout period.
	AnnotationLoadBalancerHealthCheckTimeout = "oci-load-balancer-health-check-timeout"

	// AnnotationLoadBalancerNetworkSecurityGroup is an annotation for
	// specifying Network security group Ids for the AnnotationLoadbalancer
	AnnotationLoadBalancerNetworkSecurityGroups = "oci-network-security-groups"

	// AnnotationLoadBalancerPolicy is an annotation for specifying
	// Annotationloadbalancer traffic policy("ROUND_ROBIN", "LEAST_CONNECTION", "IP_HASH")
	AnnotationLoadBalancerPolicy = "oci-load-balancer-policy"

	// Following are only applicable to Service
	// ----------------------------------------

	// ConnectionProxyProtocolVersion is the annotation used
	// on the Annotationloadbalancer to specify the proxy protocol version.
	ConnectionProxyProtocolVersion = "oci-load-balancer-connection-proxy-protocol-version"

	// AnnotationLoadBalancerBEProtocol is a  annotation for specifying the
	// load balancer listener backend protocol ("TCP", "HTTP").
	// See: https://docs.cloud.oracle.com/iaas/Content/Balance/Concepts/balanceoverview.htm#concepts
	AnnotationLoadBalancerBEProtocol = "oci-load-balancer-backend-protocol"

	// Following are only applicable to Ingress
	// ----------------------------------------

	// AnnotationLoadBalancerReservedIP is an annotation for
	// specifying a reserved IP for the load balancer.
	AnnotationLoadBalancerReservedIP = "oci-load-balancer-reserved-ip"

	// AnnotationForceHTTPSRedirect is an annotation for setting up a load balancer RuleSet for HTTP -> HTTPS 301 redirection on TLS enabled hostnames
	AnnotationForceHTTPSRedirect = "force-https-redirect"
)
