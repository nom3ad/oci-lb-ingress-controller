package controller

import (
	"context"

	"go.uber.org/zap"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	networking "k8s.io/api/networking/v1"

	ingressmanager "github.com/nom3ad/oci-lb-ingress-controller/src/manager"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// reconciler reconciles a single ingress
type reconciler struct {
	counter   map[string]int
	k8sClient client.Client
	cache     cache.Cache
	//store         store.Store
	ingressManager ingressmanager.Manager
	logger         *zap.Logger
}

func NewReconciler(controllerMgr manager.Manager, ingressMgr ingressmanager.Manager, logger *zap.Logger) (reconcile.Reconciler, error) {

	return &reconciler{
		k8sClient:      controllerMgr.GetClient(),
		cache:          controllerMgr.GetCache(),
		ingressManager: ingressMgr,
		logger:         logger,
		counter:        map[string]int{},
	}, nil
}

func (r *reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	i := r.counter[request.String()] + 1
	r.counter[request.String()] = i
	logger := r.logger.Sugar().With("ingress", request.NamespacedName)
	logger.Debugf("Reconcile #%d called", i)
	ingress := &networking.Ingress{}
	if err := r.cache.Get(ctx, request.NamespacedName, ingress); err != nil {
		if !apierrors.IsNotFound(err) {
			logger.Errorf("Reconcile #%d failed: Retryable=%t | %s", i, isRetriableError(err), err)
			return reconcile.Result{}, ignoreNonRetriableError(err)
		}
		logger.Info("DeleteIngress()")
		if err := r.ingressManager.DeleteIngress(request.NamespacedName); err != nil {
			logger.Errorf("Reconcile #%d failed: Retryable=%t | %s", i, isRetriableError(err), err)
			return reconcile.Result{}, ignoreNonRetriableError(err)
		}
		delete(r.counter, request.String())
	} else {
		logger.Info("UpdateOrCreateIngress()")
		if err := r.ingressManager.UpdateOrCreateIngress(ingress); err != nil {
			logger.Errorf("Reconcile #%d failed: Retryable=%t | %s", i, isRetriableError(err), err)
			return reconcile.Result{}, ignoreNonRetriableError(err)
		}
	}
	logger.Debugf("Reconcile #%d succeeded", i)
	return reconcile.Result{}, nil
}
