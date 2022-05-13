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

package client

import (
	"context"

	"github.com/oracle/oci-go-sdk/v46/loadbalancer"
	"github.com/pkg/errors"
)

//
func (c *client) CreateRoutingPolicy(ctx context.Context, lbID string, details loadbalancer.CreateRoutingPolicyDetails) (string, error) {
	if !c.rateLimiter.Writer.TryAccept() {
		return "", RateLimitError(true, "CreateRoutingPolicy")
	}

	resp, err := c.loadbalancer.CreateRoutingPolicy(ctx, loadbalancer.CreateRoutingPolicyRequest{
		LoadBalancerId:             &lbID,
		CreateRoutingPolicyDetails: details,
		RequestMetadata:            c.requestMetadata,
	})
	// incRequestCounter(err, createVerb, loadBalancerRoutingPolicyResource)

	if err != nil {
		return "", errors.WithStack(err)
	}

	return *resp.OpcWorkRequestId, nil
}

//
func (c *client) UpdateRoutingPolicy(ctx context.Context, lbID string, name string, details loadbalancer.UpdateRoutingPolicyDetails) (string, error) {
	if !c.rateLimiter.Writer.TryAccept() {
		return "", RateLimitError(true, "CreateRoutingPolicy")
	}

	resp, err := c.loadbalancer.UpdateRoutingPolicy(ctx, loadbalancer.UpdateRoutingPolicyRequest{
		LoadBalancerId:             &lbID,
		RoutingPolicyName:          &name,
		UpdateRoutingPolicyDetails: details,
		RequestMetadata:            c.requestMetadata,
	})
	// incRequestCounter(err, updateVerb, loadBalancerRoutingPolicyResource)

	if err != nil {
		return "", errors.WithStack(err)
	}

	return *resp.OpcWorkRequestId, nil
}

//
func (c *client) DeleteRoutingPolicy(ctx context.Context, lbID string, name string) (string, error) {
	if !c.rateLimiter.Writer.TryAccept() {
		return "", RateLimitError(true, "DeleteRoutingPolicy")
	}

	resp, err := c.loadbalancer.DeleteRoutingPolicy(ctx, loadbalancer.DeleteRoutingPolicyRequest{
		LoadBalancerId:    &lbID,
		RoutingPolicyName: &name,
		RequestMetadata:   c.requestMetadata,
	})
	// incRequestCounter(err, deleteVerb, loadBalancerRoutingPolicyResource)

	if err != nil {
		return "", errors.WithStack(err)
	}

	return *resp.OpcWorkRequestId, nil
}

//
func (c *client) CreateRuleSet(ctx context.Context, lbID string, details loadbalancer.CreateRuleSetDetails) (string, error) {
	if !c.rateLimiter.Writer.TryAccept() {
		return "", RateLimitError(true, "CreateRuleSet")
	}

	resp, err := c.loadbalancer.CreateRuleSet(ctx, loadbalancer.CreateRuleSetRequest{
		LoadBalancerId:       &lbID,
		CreateRuleSetDetails: details,
		RequestMetadata:      c.requestMetadata,
	})
	// incRequestCounter(err, createVerb, loadBalancerRuleSetResource)

	if err != nil {
		return "", errors.WithStack(err)
	}

	return *resp.OpcWorkRequestId, nil
}

//
func (c *client) UpdateRuleSet(ctx context.Context, lbID string, name string, details loadbalancer.UpdateRuleSetDetails) (string, error) {
	if !c.rateLimiter.Writer.TryAccept() {
		return "", RateLimitError(true, "CreateRuleSet")
	}

	resp, err := c.loadbalancer.UpdateRuleSet(ctx, loadbalancer.UpdateRuleSetRequest{
		LoadBalancerId:       &lbID,
		RuleSetName:          &name,
		UpdateRuleSetDetails: details,
		RequestMetadata:      c.requestMetadata,
	})
	// incRequestCounter(err, updateVerb, loadBalancerRuleSetResource)

	if err != nil {
		return "", errors.WithStack(err)
	}

	return *resp.OpcWorkRequestId, nil
}

//
func (c *client) DeleteRuleSet(ctx context.Context, lbID string, name string) (string, error) {
	if !c.rateLimiter.Writer.TryAccept() {
		return "", RateLimitError(true, "DeleteRuleSet")
	}

	resp, err := c.loadbalancer.DeleteRuleSet(ctx, loadbalancer.DeleteRuleSetRequest{
		LoadBalancerId:  &lbID,
		RuleSetName:     &name,
		RequestMetadata: c.requestMetadata,
	})
	// incRequestCounter(err, deleteVerb, loadBalancerRuleSetResource)

	if err != nil {
		return "", errors.WithStack(err)
	}

	return *resp.OpcWorkRequestId, nil
}

//
func (c *client) CreateHostname(ctx context.Context, lbID string, details loadbalancer.HostnameDetails) (string, error) {
	if !c.rateLimiter.Writer.TryAccept() {
		return "", RateLimitError(true, "CreateHostname")
	}

	resp, err := c.loadbalancer.CreateHostname(ctx, loadbalancer.CreateHostnameRequest{
		LoadBalancerId:        &lbID,
		CreateHostnameDetails: loadbalancer.CreateHostnameDetails(details),
		RequestMetadata:       c.requestMetadata,
	})
	// incRequestCounter(err, createVerb, loadBalancerHostnameResource)

	if err != nil {
		return "", errors.WithStack(err)
	}

	return *resp.OpcWorkRequestId, nil
}

//
func (c *client) DeleteHostname(ctx context.Context, lbID string, name string) (string, error) {
	if !c.rateLimiter.Writer.TryAccept() {
		return "", RateLimitError(true, "DeleteHostname")
	}

	resp, err := c.loadbalancer.DeleteHostname(ctx, loadbalancer.DeleteHostnameRequest{
		LoadBalancerId:  &lbID,
		Name:            &name,
		RequestMetadata: c.requestMetadata,
	})
	// incRequestCounter(err, deleteVerb, loadBalancerHostnameResource)

	if err != nil {
		return "", errors.WithStack(err)
	}

	return *resp.OpcWorkRequestId, nil
}

//
func (c *client) DeleteCertificate(ctx context.Context, lbID string, name string) (string, error) {
	if !c.rateLimiter.Writer.TryAccept() {
		return "", RateLimitError(true, "DeleteCertificate")
	}

	resp, err := c.loadbalancer.DeleteCertificate(ctx, loadbalancer.DeleteCertificateRequest{
		LoadBalancerId:  &lbID,
		CertificateName: &name,
		RequestMetadata: c.requestMetadata,
	})
	// incRequestCounter(err, deleteVerb, certificateResource)

	if err != nil {
		return "", errors.WithStack(err)
	}

	return *resp.OpcWorkRequestId, nil
}
