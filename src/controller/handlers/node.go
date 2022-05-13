package handlers

import (
	"context"
	"fmt"
	"strings"

	ingress "github.com/nom3ad/oci-lb-ingress-controller/src/ingress"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	networking "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// When nodes change (added or removed from cluster) update the state of the load balancers for all ingress objects

func NewNodeEventHandler(cache cache.Cache, logger *zap.Logger) handler.EventHandler {
	return &nodeEventHandler{
		cache:  cache,
		logger: *logger,
	}
}

type nodeEventHandler struct {
	cache  cache.Cache
	logger zap.Logger
}

func (h *nodeEventHandler) Create(evt event.CreateEvent, queue workqueue.RateLimitingInterface) {
	h.enqueueImpactedIngresses(queue, fmt.Sprintf("NodeCreateEvent %s", evt.Object.(*corev1.Node).Name))
}

func (h *nodeEventHandler) Delete(evt event.DeleteEvent, queue workqueue.RateLimitingInterface) {
	h.enqueueImpactedIngresses(queue, fmt.Sprintf("NodeDeleteEvent %s", evt.Object.(*corev1.Node).Name))
}

func (h *nodeEventHandler) Update(evt event.UpdateEvent, queue workqueue.RateLimitingInterface) {

}

func (h *nodeEventHandler) Generic(event.GenericEvent, workqueue.RateLimitingInterface) {
}

func (h *nodeEventHandler) enqueueImpactedIngresses(queue workqueue.RateLimitingInterface, cause string) {
	ingressList := &networking.IngressList{}
	if err := h.cache.List(context.Background(), ingressList); err != nil {
		return
	}
	var ociIngressNames []string
	for _, ing := range ingressList.Items {
		if !ingress.IsOCILoadbalancerIngress(&ing) {
			continue
		}
		nName := types.NamespacedName{Namespace: ing.Namespace, Name: ing.Name}
		ociIngressNames = append(ociIngressNames, nName.String())
		queue.Add(reconcile.Request{NamespacedName: nName})

	}
	h.logger.Sugar().Debugf("Enqueue to reconcile %d ingresses: %s | Cause: %s", len(ociIngressNames), strings.Join(ociIngressNames, ","), cause)
}
