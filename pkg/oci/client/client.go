// Copyright 2018 Oracle and/or its affiliates. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Changes removed lot of definitions and renamed stuff
//
package client

import (
	"time"

	"go.uber.org/zap"
	"k8s.io/client-go/tools/cache"

	"github.com/oracle/oci-go-sdk/v46/common"
	"github.com/oracle/oci-go-sdk/v46/core"
	"github.com/oracle/oci-go-sdk/v46/loadbalancer"
	"github.com/pkg/errors"
)

// client is an interface representing the available (simplified) OCI operations
type Interface interface {
	LoadBalancer() LoadBalancerInterface
	Networking() NetworkingInterface
	Compute() ComputeInterface
}

type client struct {
	loadbalancer loadbalancer.LoadBalancerClient
	network      core.VirtualNetworkClient
	compute      core.ComputeClient

	rateLimiter     RateLimiter
	requestMetadata common.RequestMetadata

	subnetCache cache.Store
	logger      *zap.SugaredLogger
}

// New constructs an OCI API client.
func New(logger *zap.SugaredLogger, cp common.ConfigurationProvider, opRateLimiter *RateLimiter) (Interface, error) {
	lbClient, err := loadbalancer.NewLoadBalancerClientWithConfigurationProvider(cp)
	if err != nil {
		return nil, err
	}
	network, err := core.NewVirtualNetworkClientWithConfigurationProvider(cp)
	if err != nil {
		return nil, err
	}

	compute, err := core.NewComputeClientWithConfigurationProvider(cp)
	if err != nil {
		return nil, errors.Wrap(err, "NewComputeClientWithConfigurationProvider")
	}

	return &client{
		loadbalancer: lbClient,
		network:      network,
		compute:      compute,

		rateLimiter: *opRateLimiter,
		requestMetadata: common.RequestMetadata{
			RetryPolicy: newRetryPolicy(),
		},
		subnetCache: cache.NewTTLStore(subnetCacheKeyFn, time.Duration(24)*time.Hour),
		logger:      logger,
	}, nil
}

func (c *client) LoadBalancer() LoadBalancerInterface {
	return c
}

func (c *client) Networking() NetworkingInterface {
	return c
}
func (c *client) Compute() ComputeInterface {
	return c
}

// func (c *client) ListLoadBalancers(ctx context.Context, compartmentID string) ([]loadbalancer.LoadBalancer, error) {
// 	var page *string
// 	var result []loadbalancer.LoadBalancer
// 	for {
// 		resp, err := c.loadbalancer.ListLoadBalancers(ctx, loadbalancer.ListLoadBalancersRequest{
// 			CompartmentId:  common.String(compartmentID),
// 			LifecycleState: loadbalancer.LoadBalancerLifecycleStateActive,
// 			Page:           page,
// 		})

// 		if err != nil {
// 			return nil, err
// 		}
// 		for _, lb := range resp.Items {
// 			result = append(result, lb)
// 		}

// 		if page = resp.OpcNextPage; page == nil {
// 			break
// 		}
// 	}

// 	return result, nil
// }

// func (c *client) CreateLoadBalancer(ctx context.Context, details loadbalancer.CreateLoadBalancerDetails) (loadbalancer.LoadBalancer, error) {
// 	req := loadbalancer.CreateLoadBalancerRequest{
// 		CreateLoadBalancerDetails: details,
// 		RequestMetadata:           c.requestMetadata,
// 	}
// 	if resp, err := c.loadbalancer.CreateLoadBalancer(ctx, req); err != nil {
// 		return nil, errors.Wrap(err, "creating load balancer")
// 	} else {
// 		if wr, err := c.AwaitLoadbalancerWorkRequest(ctx, *resp.OpcRequestId); err != nil {
// 			return nil, errors.Wrap(err, "Waiting for work request")
// 		}
// 	}
// }

// func (c *client) AwaitLoadbalancerWorkRequest(ctx context.Context, workrequestId string) (*loadbalancer.WorkRequest, error) {
// 	const workRequestPollInterval = 5 * time.Second
// 	var wr *loadbalancer.WorkRequest
// 	err := wait.PollUntil(workRequestPollInterval, func() (done bool, err error) {
// 		twr, err := c.loadbalancer.GetWorkRequest(ctx, workrequestId)
// 		if err != nil {
// 			if IsRetryableError(err) {
// 				return false, nil
// 			}
// 			return true, errors.WithStack(err)
// 		}
// 		switch twr.LifecycleState {
// 		case loadbalancer.WorkRequestLifecycleStateSucceeded:
// 			wr = twr
// 			return true, nil
// 		case loadbalancer.WorkRequestLifecycleStateFailed:
// 			return false, errors.Errorf("WorkRequest %q failed: %s", workrequestId, *twr.Message)
// 		}
// 		return false, nil
// 	}, ctx.Done())
// 	return wr, err
// }

// func (c *client) DeleteListener(ctx context.Context, lbID, name string) (string, error) {
// 	resp, err := c.loadbalancer.DeleteListener(ctx, loadbalancer.DeleteListenerRequest{
// 		LoadBalancerId:  &lbID,
// 		ListenerName:    &name,
// 		RequestMetadata: c.requestMetadata,
// 	})

// 	if err != nil {
// 		return "", errors.WithStack(err)
// 	}

// 	return *resp.OpcWorkRequestId, nil
// }

// func (c *client) UpdateLoadBalancerShape(ctx context.Context, lbID string, lbShapeDetails loadbalancer.UpdateLoadBalancerShapeDetails) (string, error) {
// 	resp, err := c.loadbalancer.UpdateLoadBalancerShape(ctx, loadbalancer.UpdateLoadBalancerShapeRequest{
// 		LoadBalancerId:                 &lbID,
// 		UpdateLoadBalancerShapeDetails: lbShapeDetails,
// 	})
// 	if err != nil {
// 		return "", errors.WithStack(err)
// 	}

// 	return *resp.OpcWorkRequestId, nil
// }

// func (c *client) UpdateNetworkSecurityGroups(ctx context.Context, lbID string, lbNetworkSecurityGroupDetails loadbalancer.UpdateNetworkSecurityGroupsDetails) (string, error) {
// 	resp, err := c.loadbalancer.UpdateNetworkSecurityGroups(ctx, loadbalancer.UpdateNetworkSecurityGroupsRequest{
// 		LoadBalancerId:                     &lbID,
// 		UpdateNetworkSecurityGroupsDetails: lbNetworkSecurityGroupDetails,
// 	})
// 	if err != nil {
// 		return "", errors.WithStack(err)
// 	}

// 	return *resp.OpcWorkRequestId, nil
// }

// func (c *client) DeleteLoadBalancer(ctx context.Context, id string) (string, error) {
// 	resp, err := c.loadbalancer.DeleteLoadBalancer(ctx, loadbalancer.DeleteLoadBalancerRequest{
// 		LoadBalancerId: common.String(id),
// 	})

// 	if err != nil {
// 		return "", err
// 	}

// 	return *resp.OpcWorkRequestId, nil
// }

// func (c *client) DeleteLoadBalancerByName(ctx context.Context, compartmentID string, name string) error {
// 	lb, err := c.GetLoadBalancerByName(ctx, compartmentID, name)
// 	if err != nil || lb.Id == nil {
// 		return err
// 	}

// 	if _, err := c.DeleteLoadBalancer(ctx, *lb.Id); err != nil {
// 		return err
// 	}
// 	return nil
// }

// func (c *client) GetLoadBalancerByName(ctx context.Context, compartmentID string, name string) (*loadbalancer.LoadBalancer, error) {
// 	var page *string
// 	for {
// 		resp, err := c.loadbalancer.ListLoadBalancers(ctx, loadbalancer.ListLoadBalancersRequest{
// 			CompartmentId:  common.String(compartmentID),
// 			DisplayName:    common.String(name),
// 			LifecycleState: loadbalancer.LoadBalancerLifecycleStateActive,
// 			Page:           page,
// 		})

// 		if err != nil {
// 			return nil, err
// 		}
// 		for _, lb := range resp.Items {
// 			if *lb.DisplayName == name {
// 				return &lb, nil
// 			}
// 		}
// 		if page = resp.OpcNextPage; page == nil {
// 			break
// 		}
// 	}

// 	return nil, errNotFound
// }
