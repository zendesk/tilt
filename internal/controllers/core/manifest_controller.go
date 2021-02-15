/*
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package core

import (
	"context"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tilt-dev/tilt/internal/engine"
	"github.com/tilt-dev/tilt/internal/store"
	manifests "github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/model"
)

// ManifestReconciler reconciles a Manifest object
type ManifestReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
	Store  store.RStore
}

// +kubebuilder:rbac:groups=,resources=manifests,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=,resources=manifests/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=,resources=manifests/finalizers,verbs=update

func (r *ManifestReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("manifest", req.NamespacedName)

	var apiManifest manifests.Manifest
	if err := r.Get(ctx, req.NamespacedName, &apiManifest); err != nil {
		log.Error(err, "unable to fetch Manifest")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	mn := model.ManifestName(req.Name)

	log.Info("syncing manifest in Tilt store", "name", req.Name)
	m := model.Manifest{
		Name: mn,
		DeployTarget: model.NewLocalTarget(
			model.TargetName("fake-target"),
			model.ToUnixCmd("echo hi"),
			model.Cmd{},
			nil),
	}

	r.Store.Dispatch(engine.APIServerSyncAction{
		Manifest: m,
	})

	return ctrl.Result{}, nil
}

func (r *ManifestReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&manifests.Manifest{}).
		Owns(&manifests.Manifest{}).
		Complete(r)
}
