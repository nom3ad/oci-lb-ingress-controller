package manager

import (
	"context"
	"fmt"
	"regexp"
	"sync"

	"github.com/nom3ad/oci-lb-ingress-controller/pkg/cloudprovider/providers/oci"
	ociclient "github.com/nom3ad/oci-lb-ingress-controller/pkg/oci/client"
	"github.com/nom3ad/oci-lb-ingress-controller/src/configholder"
	"github.com/nom3ad/oci-lb-ingress-controller/src/ingress"
	"github.com/nom3ad/oci-lb-ingress-controller/src/utils"
	"github.com/oracle/oci-go-sdk/v46/loadbalancer"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	networking "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/types"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// Manager maps Kubernetes Ingress objects to OCI load balancers.
type Manager interface {
	UpdateOrCreateIngress(ingress *networking.Ingress) error
	DeleteIngress(namespacedName types.NamespacedName) error
}

// ociIngressManager wraps logic for create,update,delete load balancers in OCI.
type lbManager struct {
	mu        sync.Mutex
	client    ociclient.Interface
	conf      configholder.ConfigHolder
	logger    *zap.SugaredLogger
	k8sClient k8sclient.Client
	dummyCp   *oci.CloudProvider
}

// New will create a new OCILoadBalancerController
func New(ociClient ociclient.Interface, conf configholder.ConfigHolder, controllerMgr manager.Manager, dummyCp *oci.CloudProvider, logger *zap.Logger) Manager {
	return &lbManager{
		client:    ociClient,
		conf:      conf,
		k8sClient: controllerMgr.GetClient(),
		logger:    logger.Sugar().Named("manager"),
		dummyCp:   dummyCp,
	}
}

// GetLoadBalancerByName will fetch a load balancer with a given display name if it exists
func (mgr *lbManager) tryGetLoadBalancerByNamespacedName(ctx context.Context, namespacedName types.NamespacedName, logger *zap.SugaredLogger) (*loadbalancer.LoadBalancer, error) {
	loadBalancerName := ingress.GetLoadBalancerName(namespacedName.Namespace, namespacedName.Name)
	compartmentID := mgr.conf.GetCompartmentId()
	logger.With("loadBalancerName", loadBalancerName).With("compartment", compartmentID).Debug("Get LB by name")
	if lb, err := mgr.client.LoadBalancer().GetLoadBalancerByName(ctx, compartmentID, loadBalancerName); err != nil {
		// { "code": "NotAuthorizedOrNotFound", "message": "Authorization failed or requested resource not found.", "status": 404 }
		// if ociclient.IsNotFound(err) { //! will give 404 even auth failed
		// 	return nil, nil
		// }
		if errors.Cause(err).Error() == "not found" {
			return nil, nil
		}
		return nil, err
	} else {
		return lb, nil
	}
}

// DeleteLoadBalancerByName will delete a load balancer with a matching display name if it exists in OCI.
func (mgr *lbManager) DeleteIngress(namespacedName types.NamespacedName) error {
	ctx := context.Background()
	logger := mgr.logger.With("ingress", namespacedName)
	lb, err := mgr.tryGetLoadBalancerByNamespacedName(ctx, namespacedName, logger)
	if err != nil {
		logger.With(zap.Error(err)).Error("Failed tryGetLoadBalancerByNamespacedName()")
		return err
	}
	if lb == nil {
		logger.Warnf("No loadbalancer exists for %s to delete", namespacedName)
		return nil
	}
	id := *lb.Id
	name := *lb.DisplayName
	logger = logger.With("loadBalancerID", id, "loadBalancerName", name)
	logger.Info("Deleting LB")
	workReqID, err := mgr.client.LoadBalancer().DeleteLoadBalancer(ctx, *lb.Id)
	if err != nil {
		if ociclient.IsNotFound(err) {
			logger.Warn("Loadbalancer seems to be already deleted!")
			return nil
		}
		logger.With(zap.Error(err)).Error("Failed to delete loadbalancer")
		return errors.Wrapf(err, "delete load balancer %s|%s", name, id)
	}
	_, err = mgr.client.LoadBalancer().AwaitWorkRequest(ctx, workReqID)
	if err != nil {
		logger.With(zap.Error(err)).Error("Timeout waiting for loadbalancer delete")
		return errors.Wrapf(err, "awaiting deletion of load balancer %s|%q", name, name)
	}
	logger.Info("Successfully deleted LB")
	return nil
}

// UpdateOrCreateIngress creates/update ingress based on OCI LB
func (mgr *lbManager) UpdateOrCreateIngress(ing *networking.Ingress) error {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	ctx := context.Background()
	namespacedName := types.NamespacedName{Namespace: ing.Namespace, Name: ing.Name}
	logger := mgr.logger.With("ingress", namespacedName)

	spec, err := ingress.NewIngressLBSpec(mgr.conf, ing, mgr.client, mgr.k8sClient, logger.Desugar())
	if err != nil {
		return errors.Wrap(err, "Couldn't derive LB spec from ingress")
	}
	lb, err := mgr.tryGetLoadBalancerByNamespacedName(ctx, namespacedName, logger)
	if err != nil {
		logger.With(zap.Error(err)).Error("Failed tryGetLoadBalancerByNamespacedName()")
		return err
	}
	exists := lb != nil //! TODO: fix upstream: !ociclient.IsNotFound(err)

	if !exists {
		if lb, err = mgr.createLoadBalancer(ctx, spec); err != nil {
			return errors.Wrap(err, "Failed to create Loadbalancer")
		}
		// create follows an update to update all associations
		if lb, err = mgr.updateLoadBalancer(ctx, lb, spec); err != nil {
			return errors.Wrap(err, "Failed to update newly created Loadbalancer")
		}
	} else {
		if lb.LifecycleState == "FAILED" {
			return errors.Errorf("Lb %s (%s) is in FAILED state. Cant update. Need to delete manually", *lb.Id, *lb.DisplayName)
			// TODO: Should we try delete to delete LB?
		}
		if lb.LifecycleState == "DELETING" {
			return errors.Errorf("Lb %s (%s) is being deleted. Cant update it", *lb.Id, *lb.DisplayName)
		}
		if lb, err = mgr.updateLoadBalancer(ctx, lb, spec); err != nil {
			return errors.Wrap(err, "Failed to update existing Loadbalancer")
		}
	}
	if err := mgr.updateIngressStatus(ing, lb); err != nil {
		return errors.Wrap(err, "Failed to update ingress status")
	}
	return nil
}

func (mgr *lbManager) updateIngressStatus(ingress *networking.Ingress, lb *loadbalancer.LoadBalancer) error {
	if len(lb.IpAddresses) == 0 || lb.IpAddresses[0].IpAddress == nil {
		return fmt.Errorf("could not update Ingres status: No IP found")
	}
	ingress.Status.LoadBalancer.Ingress = []corev1.LoadBalancerIngress{
		{
			IP: *lb.IpAddresses[0].IpAddress,
			// Hostname: *lb.IpAddresses[0],
		},
	}
	if err := mgr.k8sClient.Status().Update(context.Background(), ingress); err != nil {
		return fmt.Errorf("could not update Ingres status: %v", err)
	}
	return nil
}

// https://github.dev/oracle/oci-cloud-controller-manager
func (mgr *lbManager) createLoadBalancer(ctx context.Context, spec *ingress.IngressLBSpec) (*loadbalancer.LoadBalancer, error) {
	logger := mgr.logger.With("loadBalancerName", spec.Name)
	createDetails := loadbalancer.CreateLoadBalancerDetails{
		CompartmentId: utils.PtrToString(mgr.conf.GetCompartmentId()),
		DisplayName:   &spec.Name,
		ShapeName:     &spec.Shape,
		IsPrivate:     &spec.Internal,
		SubnetIds:     spec.Subnets,
		Listeners:     spec.Listeners,
		BackendSets:   spec.BackendSets,
		Hostnames:     spec.HostnameDetails,
		Certificates:  spec.Certificates,
		// IpMode:                  loadbalancer.CreateLoadBalancerDetailsIpModeIpv4,
		NetworkSecurityGroupIds: spec.NetworkSecurityGroupIds,
		FreeformTags: map[string]string{
			"IngressName":      spec.Ingress.Name,
			"IngressNamespace": spec.Ingress.Namespace,
			"IngressUID":       string(spec.Ingress.UID),
		},
		RuleSets: spec.RuleSets,
	}
	listeners := map[string]loadbalancer.ListenerDetails{}
	//XXX: Workaround #1:
	// Since routingPolicies are not yet created and loadbalancer.CreateLoadBalancer() OCI API don't support it to be accepted via loadbalancer.CreateLoadBalancerDetails
	// So we need to create LB by removing routingPolicies associations from listeners and then update them after creating routingPolicies
	// Seems routingPolicies is the only one having this problem. In future version of OCI api might support this.

	for lName, listener := range spec.Listeners {
		listener.RoutingPolicyName = nil
		listeners[lName] = listener
	}
	createDetails.Listeners = listeners

	if spec.IsFlexibleShape() {
		createDetails.ShapeDetails = &loadbalancer.ShapeDetails{
			MinimumBandwidthInMbps: spec.FlexMin,
			MaximumBandwidthInMbps: spec.FlexMax,
		}
	}

	if spec.LoadBalancerIP != "" {
		logger.Infof("GetReservedIpOcidByIpAddress(%s)", spec.LoadBalancerIP)
		reservedIpOCID, err := oci.GetReservedIpOcidByIpAddress(ctx, spec.LoadBalancerIP, mgr.client.Networking())
		if err != nil {
			return nil, err
		}
		createDetails.ReservedIps = []loadbalancer.ReservedIp{
			{
				Id: reservedIpOCID,
			},
		}
	}
	logger.Info("Create a new LB: " + regexp.MustCompile(`-----BEGIN[\s\w\\+/=-]+-----END`).ReplaceAllString(utils.Jsonify(createDetails), "-----BEGIN ***** -----END"))
	wrID, err := mgr.client.LoadBalancer().CreateLoadBalancer(ctx, createDetails)
	if err != nil {
		return nil, err
	}
	logger.With("wrID", wrID).Info("Awaiting work request completion")
	wr, err := mgr.client.LoadBalancer().AwaitWorkRequest(ctx, wrID)
	if err != nil {
		return nil, errors.Wrap(err, "awaiting load balancer")
	}

	lb, err := mgr.client.LoadBalancer().GetLoadBalancer(ctx, *wr.LoadBalancerId)
	if err != nil {
		return nil, errors.Wrapf(err, "get load balancer %q", *wr.LoadBalancerId)
	}
	lbOcid := *lb.Id
	logger = logger.With("loadBalancerID", lbOcid)
	logger.Info("LB created")
	return lb, nil
}

func (mgr *lbManager) updateLoadBalancer(ctx context.Context, lb *loadbalancer.LoadBalancer, spec *ingress.IngressLBSpec) (*loadbalancer.LoadBalancer, error) {
	logger := mgr.logger.With("loadBalancerID", *lb.Id).With("loadBalancerName", lb.DisplayName)

	ad := &ActionDispatcher{ctx: ctx, logger: logger}
	mgr.enqueueRoutingPoliciesActions(ad, lb, spec)
	mgr.enqueueRuleSetsActions(ad, lb, spec)
	mgr.enqueueHostnameActions(ad, lb, spec)
	mgr.enqueueCertificateActions(ad, lb, spec)

	// FIXME: updated routingPolicy might contain a rule referencing non existing BackendSet. Ensure that backend sets are created
	// error is suppressed
	func() {
		_ = mgr.dummyCp.UpdateLoadBalancer(ctx, lb, &spec.LBSpec)
		lb, _ = mgr.client.LoadBalancer().GetLoadBalancer(ctx, *lb.Id)
	}()

	if err := ad.Run(CreateAction); err != nil { // routingPolicies, hostnameNames, rulesets
		return nil, err
	}

	if err := ad.Run(UpdateAction, "routingpolicy"); err != nil {
		return nil, err
	}
	if err := mgr.dummyCp.UpdateLoadBalancer(ctx, lb, &spec.LBSpec); err != nil { // Listener, BackendSet
		return nil, err
	}
	if err := ad.Run(UpdateAction); err != nil {
		return nil, err
	}
	if err := ad.Run(DeleteAction); err != nil {
		return nil, err
	}
	return lb, nil
}

func (mgr *lbManager) enqueueRoutingPoliciesActions(ad *ActionDispatcher, lb *loadbalancer.LoadBalancer, spec *ingress.IngressLBSpec) {
	lbOcid := *lb.Id
	ctx := ad.Context()
	logger := ad.Logger()
	toBeCreated, toBeRemoved, toBeUpdated := utils.MapCompare(spec.RoutingPolicies, lb.RoutingPolicies, func(fromSpec, fromLb interface{}) bool {
		return utils.NullOrDeepEqual(fromSpec, fromLb)
	})
	logger.Debugf("RoutingPolicies: toBeCreated=%v toBeRemoved=%v toBeUpdated=%v", toBeCreated.List(), toBeRemoved.List(), toBeUpdated.List())

	patchLbInfo := func(policyName string, present bool) {
		// After update, applying routing policy changes to the LB Info to save a GetLoadbalancer() API call
		if lb.RoutingPolicies == nil {
			lb.RoutingPolicies = map[string]loadbalancer.RoutingPolicy{}
		}
		if present {
			lb.RoutingPolicies[policyName] = spec.RoutingPolicies[policyName]
		} else {
			delete(lb.RoutingPolicies, policyName)
		}
	}

	for policyName_ := range toBeCreated {
		policyName := policyName_
		requiredPolicy := spec.RoutingPolicies[policyName]
		ad.AddFunc(CreateAction, "routingpolicy", func() error {
			logger.Infof("Creating routingpolicy %q | %+v", policyName, requiredPolicy.Rules)
			createRoutingPolicyDetails := loadbalancer.CreateRoutingPolicyDetails{
				Name:                     utils.PtrToString(policyName),
				ConditionLanguageVersion: loadbalancer.CreateRoutingPolicyDetailsConditionLanguageVersionEnum(requiredPolicy.ConditionLanguageVersion),
				Rules:                    requiredPolicy.Rules,
			}
			logger.Debugf("CreateRoutingPolicyDetails: %s", utils.Jsonify(createRoutingPolicyDetails))
			wrID, err := mgr.client.LoadBalancer().CreateRoutingPolicy(ctx, lbOcid, createRoutingPolicyDetails)
			return mgr.awaitRequest(ctx, wrID, err, func() { patchLbInfo(policyName, true) }, "create routingpolicy %q", policyName)
		})
	}

	for policyName_ := range toBeUpdated {
		policyName := policyName_
		requiredPolicy := spec.RoutingPolicies[policyName]
		ad.AddFunc(UpdateAction, "routingpolicy", func() error {
			logger.Infof("Updating existing routingpolicy %q", policyName)
			updateRoutingPolicyDetails := loadbalancer.UpdateRoutingPolicyDetails{
				Rules:                    requiredPolicy.Rules,
				ConditionLanguageVersion: loadbalancer.UpdateRoutingPolicyDetailsConditionLanguageVersionEnum(requiredPolicy.ConditionLanguageVersion),
			}
			logger.Debugf("UpdateRoutingPolicyDetails: %s", utils.Jsonify(updateRoutingPolicyDetails))
			wrID, err := mgr.client.LoadBalancer().UpdateRoutingPolicy(ctx, lbOcid, policyName, updateRoutingPolicyDetails)
			return mgr.awaitRequest(ctx, wrID, err, func() { patchLbInfo(policyName, true) }, "update routingpolicy %q", policyName)
		})
	}

	for policyName_ := range toBeRemoved {
		policyName := policyName_
		ad.AddFunc(DeleteAction, "routingpolicy", func() error {
			logger.Infof("Deleting existing routingpolicy %q", policyName)
			wrID, err := mgr.client.LoadBalancer().DeleteRoutingPolicy(ctx, lbOcid, policyName)
			return mgr.awaitRequest(ctx, wrID, err, func() { patchLbInfo(policyName, false) }, "delete routingpolicy %q", policyName)
		})
	}

}

func (mgr *lbManager) enqueueRuleSetsActions(ad *ActionDispatcher, lb *loadbalancer.LoadBalancer, spec *ingress.IngressLBSpec) {
	lbOcid := *lb.Id
	ctx := ad.Context()
	logger := ad.Logger()
	toBeCreated, toBeRemoved, toBeUpdated := utils.MapCompare(spec.RuleSets, lb.RuleSets, func(fromSpec, fromLb interface{}) bool {
		return utils.StructsAreEqualForKeys(fromSpec, fromLb, "Items")
	})
	logger.Debugf("RuleSets: toBeCreated=%v toBeRemoved=%v toBeUpdated=%v", toBeCreated.List(), toBeRemoved.List(), toBeUpdated.List())

	patchLbInfo := func(ruleSetName string, present bool) {
		// After update, applying ruleSet changes to the LB Info to save a GetLoadbalancer() API call
		if lb.RuleSets == nil {
			lb.RuleSets = map[string]loadbalancer.RuleSet{}
		}
		if present {
			lb.RuleSets[ruleSetName] = loadbalancer.RuleSet{Name: &ruleSetName, Items: spec.RuleSets[ruleSetName].Items}
		} else {
			delete(lb.RuleSets, ruleSetName)
		}
	}

	for ruleSetName_ := range toBeCreated {
		ruleSetName := ruleSetName_
		requiredRuleSet := spec.RuleSets[ruleSetName]
		ad.AddFunc(CreateAction, "ruleSet", func() error {
			logger.Infof("Creating ruleSet %q | %+v", ruleSetName, requiredRuleSet.Items)
			createRuleSetDetails := loadbalancer.CreateRuleSetDetails{
				Name:  utils.PtrToString(ruleSetName),
				Items: requiredRuleSet.Items,
			}
			logger.Debugf("CreateRuleSetDetails: %s", utils.Jsonify(createRuleSetDetails))
			wrID, err := mgr.client.LoadBalancer().CreateRuleSet(ctx, lbOcid, createRuleSetDetails)
			return mgr.awaitRequest(ctx, wrID, err, func() { patchLbInfo(ruleSetName, true) }, "create RuleSet %q", ruleSetName)
		})
	}

	for ruleSetName_ := range toBeUpdated {
		ruleSetName := ruleSetName_
		requiredRuleSet := spec.RuleSets[ruleSetName]
		ad.AddFunc(UpdateAction, "ruleSet", func() error {
			logger.Infof("Updating existing ruleSet %q", ruleSetName)
			updateRuleSetDetails := loadbalancer.UpdateRuleSetDetails(requiredRuleSet)
			logger.Debugf("UpdateRuleSetDetails: %s", utils.Jsonify(updateRuleSetDetails))
			wrID, err := mgr.client.LoadBalancer().UpdateRuleSet(ctx, lbOcid, ruleSetName, updateRuleSetDetails)
			return mgr.awaitRequest(ctx, wrID, err, func() { patchLbInfo(ruleSetName, true) }, "update ruleSet %q", ruleSetName)
		})
	}

	for ruleSetName_ := range toBeRemoved {
		ruleSetName := ruleSetName_
		ad.AddFunc(DeleteAction, "ruleSet", func() error {
			logger.Infof("Deleting existing ruleSet %q", ruleSetName)
			wrID, err := mgr.client.LoadBalancer().DeleteRuleSet(ctx, lbOcid, ruleSetName)
			return mgr.awaitRequest(ctx, wrID, err, func() { patchLbInfo(ruleSetName, false) }, "delete ruleSet %q", ruleSetName)
		})
	}
}

func (mgr *lbManager) enqueueHostnameActions(ad *ActionDispatcher, lb *loadbalancer.LoadBalancer, spec *ingress.IngressLBSpec) {
	lbOcid := *lb.Id
	ctx := ad.Context()
	logger := ad.Logger()
	// Since HostnameName and hostname has 1-1 mapping, intersection result corresponds to unchanged items
	toBeCreated, toBeRemoved, unchanged := utils.MapCompare(spec.HostnameDetails, lb.Hostnames, nil)
	logger.Debugf("Hostnames: toBeCreated=%v toBeRemoved=%v unchanged=%v", toBeCreated.List(), toBeRemoved.List(), unchanged.List())

	patchLbInfo := func(hostnameName string, present bool) {
		// After update, applying hostname changes to the LB Info to save a GetLoadbalancer() API call
		if lb.Hostnames == nil {
			lb.Hostnames = map[string]loadbalancer.Hostname{}
		}
		if present {
			lb.Hostnames[hostnameName] = loadbalancer.Hostname(spec.HostnameDetails[hostnameName])
		} else {
			delete(lb.Hostnames, hostnameName)
		}
	}

	for hostnameName_ := range toBeCreated {
		hostnameName := hostnameName_
		ad.AddFunc(CreateAction, "hostname", func() error {
			logger.Infof("Creating hostname %q", hostnameName)
			wrID, err := mgr.client.LoadBalancer().CreateHostname(ctx, lbOcid, spec.HostnameDetails[hostnameName])
			return mgr.awaitRequest(ctx, wrID, err, func() { patchLbInfo(hostnameName, true) }, "create hostname %q", hostnameName)
		})
	}

	for hostnameName_ := range toBeRemoved {
		hostnameName := hostnameName_
		ad.AddFunc(DeleteAction, "hostname", func() error {
			logger.Infof("Removing hostname %q", hostnameName)
			// TODO check if it is used in any listener, if so remove the listener.  It will be created back later when called updateListeners()
			wrID, err := mgr.client.LoadBalancer().DeleteHostname(ctx, lbOcid, hostnameName)
			return mgr.awaitRequest(ctx, wrID, err, func() { patchLbInfo(hostnameName, false) }, "delete hostname %q", hostnameName)
		})
	}

}

func (mgr *lbManager) enqueueCertificateActions(ad *ActionDispatcher, lb *loadbalancer.LoadBalancer, spec *ingress.IngressLBSpec) {
	lbOcid := *lb.Id
	ctx := ad.Context()
	logger := ad.Logger()
	// toBeUpdated (intersection list) should be empty always, as certificate name consists of a hash derived from certificate contents.
	toBeCreated, toBeRemoved, toBeUpdated := utils.MapCompare(spec.Certificates, lb.Certificates, func(fromSpec, fromLb interface{}) bool {
		return utils.StructsAreEqualForKeys(fromSpec, fromLb, "PublicCertificate", "CaCertificate")
	})
	logger.Debugf("Certificates: toBeCreated=%v toBeRemoved=%v toBeUpdated=%v", toBeCreated.List(), toBeRemoved.List(), toBeUpdated.List())
	if toBeUpdated.Len() != 0 {
		// there is no way to update existing certificate. Only public key is readable.
		panic("Update to existing certificate is not possible")
	}

	patchLbInfo := func(certName string, present bool) {
		// After update, applying certificate changes to the LB Info to save a GetLoadbalancer() API call
		if lb.Certificates == nil {
			lb.Certificates = map[string]loadbalancer.Certificate{}
		}
		if present {
			certDetails := spec.Certificates[certName]
			lb.Certificates[certName] = loadbalancer.Certificate{
				CertificateName:   certDetails.CertificateName,
				PublicCertificate: certDetails.PublicCertificate,
				CaCertificate:     certDetails.CaCertificate}
		} else {
			delete(lb.Certificates, certName)
		}
	}

	for certName_ := range toBeCreated {
		certName := certName_
		requiredCert := spec.Certificates[certName]
		ad.AddFunc(CreateAction, "certificate", func() error {
			// stringify certificate
			logger.Infof("Creating certificate %q", certName)
			wrID, err := mgr.client.LoadBalancer().CreateCertificate(ctx, lbOcid, requiredCert)
			return mgr.awaitRequest(ctx, wrID, err, func() { patchLbInfo(certName, true) }, "create certificate %q", certName)
		})
	}
	for certName_ := range toBeRemoved {
		certName := certName_
		ad.AddFunc(DeleteAction, "certificate", func() error {
			logger.Infof("Deleting existing certificate %q", certName)
			wrID, err := mgr.client.LoadBalancer().DeleteCertificate(ctx, lbOcid, certName)
			return mgr.awaitRequest(ctx, wrID, err, func() { patchLbInfo(certName, false) }, "delete certificate %q", certName)
		})
	}
}

func (mgr *lbManager) awaitRequest(ctx context.Context, wrID string, err error, onSuccess func(), fmt string, args ...interface{}) error {
	if err != nil {
		return errors.Wrapf(err, fmt, args...)
	}
	_, err = mgr.client.LoadBalancer().AwaitWorkRequest(ctx, wrID)
	if err != nil {
		return errors.Wrapf(err, "await:"+fmt, args...)
	}
	if onSuccess != nil {
		onSuccess()
	}
	return nil

}
