package configholder

import (
	"strings"

	providercfg "github.com/nom3ad/oci-lb-ingress-controller/pkg/cloudprovider/providers/oci/config"
)

var DefaultLoadBalancerSubnetIds string

type configHolder struct {
	_conf providercfg.Config
}

func (c *configHolder) GetCompartmentId() string {
	return c._conf.CompartmentID
}

func (c *configHolder) GetSubnetIds() (subnetIds []string) {
	if c._conf.LoadBalancer != nil {
		if c._conf.LoadBalancer.Subnet1 != "" {
			subnetIds = append(subnetIds, c._conf.LoadBalancer.Subnet1)
		}
		if c._conf.LoadBalancer.Subnet2 != "" {
			subnetIds = append(subnetIds, c._conf.LoadBalancer.Subnet2)
		}
		return
	}
	if DefaultLoadBalancerSubnetIds != "" {
		for _, s := range strings.SplitN(DefaultLoadBalancerSubnetIds, ",", 2) {
			s = strings.TrimSpace(s)
			if s != "" {
				subnetIds = append(subnetIds, s)
			}

		}
	}
	return
}

func NewConfigHolder(conf *providercfg.Config) ConfigHolder {
	return &configHolder{
		_conf: *conf,
	}
}
