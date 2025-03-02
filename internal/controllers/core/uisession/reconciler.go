package uisession

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/tilt-dev/tilt/internal/hud/server"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

// The uisession.Reconciler is not a real reconciler because UIResource is not
// a real API object.
//
// It's a fake status object that reports the Status of the legacy engine. The
// uisession.Reconciler wathces that status and broadcasts it to the legacy web
// UI.
type Reconciler struct {
	client ctrlclient.Client
	wsList *server.WebsocketList
}

var _ reconcile.Reconciler = &Reconciler{}

func NewReconciler(wsList *server.WebsocketList) *Reconciler {
	return &Reconciler{wsList: wsList}
}

func (r *Reconciler) SetClient(client ctrlclient.Client) {
	r.client = client
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	session := &v1alpha1.UISession{}
	err := r.client.Get(ctx, req.NamespacedName, session)
	if err != nil && !apierrors.IsNotFound(err) {
		return ctrl.Result{}, fmt.Errorf("uisession reconcile: %v", err)
	}

	if apierrors.IsNotFound(err) || session.ObjectMeta.DeletionTimestamp != nil {
		// NOTE(nick): This should never happen, and if it does, Tilt should
		// immediately re-create the session.
		return ctrl.Result{}, nil
	}

	// Broadcast to all websockets.
	r.wsList.ForEach(func(ws *server.WebsocketSubscriber) {
		ws.SendUISessionUpdate(ctx, session)
	})

	return ctrl.Result{}, nil
}

func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.UISession{}).
		Complete(r)
}
