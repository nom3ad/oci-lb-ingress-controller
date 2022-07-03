package handlers

import (
	"github.com/nom3ad/oci-lb-ingress-controller/src/ingress"
	"go.uber.org/zap"
	networking "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func NewIngressEventHandler(cache cache.Cache, logger *zap.Logger) handler.EventHandler {
	return &ingressEventHandler{
		cache:  cache,
		logger: logger,
	}
}

type ingressEventHandler struct {
	cache  cache.Cache
	logger *zap.Logger
}

// Create is a handler called when an ingress object is created
func (h *ingressEventHandler) Create(evt event.CreateEvent, queue workqueue.RateLimitingInterface) {
	if ing, ok := evt.Object.(*networking.Ingress); ok && ing != nil {
		h.enqueueIfIngressClassMatched(ing, queue, "IngressCreateEvent")
	} else {
		h.logger.Sugar().Warn("CreateEvent received with no ingress object", evt)
	}
}

// Update is a handler called when an ingress object is updated
func (h *ingressEventHandler) Update(evt event.UpdateEvent, queue workqueue.RateLimitingInterface) {
	if oldIng, ok := evt.ObjectNew.(*networking.Ingress); ok && oldIng != nil {
		h.enqueueIfIngressClassMatched(oldIng, queue, "IngressUpdateEvent")
	} else {
		h.logger.Sugar().Warn("UpdateEvent received with no old ingress object", evt)
	}

	if newIng, ok := evt.ObjectNew.(*networking.Ingress); ok && newIng != nil {
		h.enqueueIfIngressClassMatched(newIng, queue, "IngressUpdateEvent")
	} else {
		h.logger.Sugar().Warn("UpdateEvent received with no new ingress object", evt)
	}
}

// Delete is a handler called when an ingress object is deleted
func (h *ingressEventHandler) Delete(evt event.DeleteEvent, queue workqueue.RateLimitingInterface) {
	if ing, ok := evt.Object.(*networking.Ingress); ok && ing != nil {
		h.enqueueIfIngressClassMatched(ing, queue, "IngressDeleteEvent")
	} else {
		h.logger.Sugar().Warn("DeleteEvent received with no ingress object", evt)
	}
}

// Generic is a handler called when an ingress object is modified but none of the above
func (h *ingressEventHandler) Generic(evt event.GenericEvent, queue workqueue.RateLimitingInterface) {
	if ing, ok := evt.Object.(*networking.Ingress); ok && ing != nil {
		h.enqueueIfIngressClassMatched(ing, queue, "IngressGenericEvent")
	} else {
		h.logger.Sugar().Warn("GenericEvent received with no ingress object", evt)
	}
}

func (h *ingressEventHandler) enqueueIfIngressClassMatched(ing *networking.Ingress, queue workqueue.RateLimitingInterface, cause string) {
	nName := types.NamespacedName{Namespace: ing.Namespace, Name: ing.Name}
	if !ingress.IsOCILoadbalancerIngress(ing) {
		h.logger.Sugar().Debugf("Won't reconcile ingress %s class: %s", ingress.GetIngressClassName(ing))
		return
	}
	h.logger.Sugar().Debugf("Enqueue to reconcile ingress %s | Cause: %s", nName, cause)
	queue.Add(reconcile.Request{NamespacedName: nName})
}
