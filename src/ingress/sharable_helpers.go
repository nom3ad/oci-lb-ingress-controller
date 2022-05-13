package ingress

import (
	"fmt"
	"strconv"
	"strings"

	. "github.com/nom3ad/oci-lb-ingress-controller/pkg/cloudprovider/providers/oci"
	"github.com/pkg/errors"
	networking "k8s.io/api/networking/v1"
)

const FlexibleShapeName = "flexible" // 100Mbps in official oci-cloud-provider Loadbalancer service implementation

var DefaultLBShape = FlexibleShapeName
var DefaultFlexShapeMinMbps = 10
var DefaultFlexShapeMaxMbps = 10

const FlexShapeAbsoluteMinMbps = 10
const FlexShapeAbsoluteMaxMbps = 8192

func getLBShape(ing *networking.Ingress) (string, *int, *int, error) {
	shape := DefaultLBShape
	if s := GetAnnotation(ing, AnnotationLoadBalancerShape); s != "" {
		shape = s
	}

	// For flexible shape, LBaaS requires the ShapeName to be in lower case `flexible`
	// but they have a public documentation bug where it is mentioned as `Flexible`
	// We are converting to lowercase to check the shape name and send to LBaaS
	shapeLower := strings.ToLower(shape)

	if shapeLower == FlexibleShapeName {
		shape = FlexibleShapeName
	}

	// if it's not a flexshape LB return the ShapeName as the shape
	if shape != FlexibleShapeName {
		return shape, nil, nil, nil
	}

	var flexMinS, flexMaxS string
	var flexShapeMinMbps, flexShapeMaxMbps int

	if fmin := GetAnnotation(ing, AnnotationLoadBalancerShapeFlexMin); fmin != "" {
		flexMinS = fmin
	}

	if fmax := GetAnnotation(ing, AnnotationLoadBalancerShapeFlexMax); fmax != "" {
		flexMaxS = fmax
	}

	if flexMaxS == "" && flexMinS == "" {
		flexMinS = strconv.Itoa(DefaultFlexShapeMinMbps)
		flexMaxS = strconv.Itoa(DefaultFlexShapeMaxMbps)
	}
	if flexMinS == "" || flexMaxS == "" {
		return "", nil, nil, fmt.Errorf("error parsing service annotation: %s=flexible requires %s and %s to be set",
			AnnotationLoadBalancerShape,
			AnnotationLoadBalancerShapeFlexMin,
			AnnotationLoadBalancerShapeFlexMax,
		)
	}

	flexShapeMinMbps, err := strconv.Atoi(flexMinS)
	if err != nil {
		return "", nil, nil, errors.Wrap(err,
			fmt.Sprintf("The annotation %s should contain only integer value", AnnotationLoadBalancerShapeFlexMin))
	}
	flexShapeMaxMbps, err = strconv.Atoi(flexMaxS)
	if err != nil {
		return "", nil, nil, errors.Wrap(err,
			fmt.Sprintf("The annotation %s should contain only integer value", AnnotationLoadBalancerShapeFlexMax))
	}

	if flexShapeMinMbps < FlexShapeAbsoluteMinMbps {
		flexShapeMinMbps = FlexShapeAbsoluteMinMbps
	}
	if flexShapeMaxMbps < FlexShapeAbsoluteMinMbps {
		flexShapeMaxMbps = FlexShapeAbsoluteMinMbps
	}
	if flexShapeMinMbps > FlexShapeAbsoluteMaxMbps {
		flexShapeMinMbps = FlexShapeAbsoluteMaxMbps
	}
	if flexShapeMaxMbps > FlexShapeAbsoluteMaxMbps {
		flexShapeMaxMbps = FlexShapeAbsoluteMaxMbps
	}
	if flexShapeMaxMbps < flexShapeMinMbps {
		flexShapeMaxMbps = flexShapeMinMbps
	}

	return shape, &flexShapeMinMbps, &flexShapeMaxMbps, nil
}

func getLoadBalancerPolicy(obj AnnotatedObject) (string, error) {
	lbPolicy := GetAnnotation(obj, AnnotationLoadBalancerPolicy)
	if lbPolicy == "" {
		return DefaultLoadBalancerPolicy, nil
	}
	knownLBPolicies := map[string]struct{}{
		IPHashLoadBalancerPolicy:           {},
		LeastConnectionsLoadBalancerPolicy: {},
		RoundRobinLoadBalancerPolicy:       {},
	}

	if _, ok := knownLBPolicies[lbPolicy]; ok {
		return lbPolicy, nil
	}

	return "", fmt.Errorf("loadbalancer policy \"%s\" is not valid", GetAnnotation(obj, AnnotationLoadBalancerPolicy))
}
