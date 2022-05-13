package handlers

import (
	"context"
	"fmt"
	"strings"

	"github.com/nom3ad/oci-lb-ingress-controller/src/ingress"
	"github.com/nom3ad/oci-lb-ingress-controller/src/utils"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	networking "k8s.io/api/networking/v1"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// When secrets change (added or removed from cluster) update the state of the load balancers for all ingress objects

func NewSecretEventHandler(cache cache.Cache, logger *zap.Logger) handler.EventHandler {
	return &secretEventHandler{
		cache:  cache,
		logger: *logger,
	}
}

type secretEventHandler struct {
	cache  cache.Cache
	logger zap.Logger
}

func (h *secretEventHandler) Create(evt event.CreateEvent, queue workqueue.RateLimitingInterface) {
	secret := evt.Object.(*corev1.Secret)
	h.enqueueImpactedIngresses(queue, secret, fmt.Sprintf("SecretCreate %s", utils.GetNamespacedNameStr(secret.ObjectMeta)))
}

func (h *secretEventHandler) Delete(evt event.DeleteEvent, queue workqueue.RateLimitingInterface) {
}

func (h *secretEventHandler) Update(evt event.UpdateEvent, queue workqueue.RateLimitingInterface) {
	secret := evt.ObjectNew.(*corev1.Secret)
	h.enqueueImpactedIngresses(queue, secret, fmt.Sprintf("SecretUpdate %s", utils.GetNamespacedNameStr(secret.ObjectMeta)))
}

func (h *secretEventHandler) Generic(event.GenericEvent, workqueue.RateLimitingInterface) {
}

func (h *secretEventHandler) enqueueImpactedIngresses(queue workqueue.RateLimitingInterface, secret *corev1.Secret, cause string) {
	ingressList := &networking.IngressList{}
	if err := h.cache.List(context.Background(), ingressList); err != nil {
		return
	}
	secretNsNameString := utils.GetNamespacedNameStr(secret.ObjectMeta)
	var ociIngressNames []string
	for _, ing := range ingressList.Items {
		if !ingress.IsOCILoadbalancerIngress(&ing) {
			continue
		}
		ingressHasSecret := false
		for _, ingTls := range ing.Spec.TLS {
			if utils.AsNamespacedName(ingTls.SecretName, ing.Namespace).String() == secretNsNameString {
				ingressHasSecret = true
				break
			}
		}
		if !ingressHasSecret {
			continue
		}
		nName := utils.GetNamespacedName(ing.ObjectMeta)
		ociIngressNames = append(ociIngressNames, nName.String())
		queue.Add(reconcile.Request{NamespacedName: nName})
	}
	if len(ociIngressNames) != 0 {
		h.logger.Sugar().Debugf("Enqueue to reconcile %d ingresses: %s | Cause: %s", len(ociIngressNames), strings.Join(ociIngressNames, ","), cause)
	}
}
