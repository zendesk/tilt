package engine

import (
	"context"

	"k8s.io/apimachinery/pkg/watch"

	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/store"
)

type UIDMapManager struct {
	watching bool
	kCli     k8s.Client
}

func NewUIDMapManager(kCli k8s.Client) *UIDMapManager {
	return &UIDMapManager{kCli: kCli}
}

func (m *UIDMapManager) needsWatch(st store.RStore) bool {
	if !k8sEventsFeatureFlagOn() {
		return false
	}

	state := st.RLockState()
	defer st.RUnlockState()

	atLeastOneK8s := false
	for _, m := range state.Manifests() {
		if m.IsK8s() {
			atLeastOneK8s = true
		}
	}
	return atLeastOneK8s && state.WatchFiles && !m.watching
}

func (m *UIDMapManager) OnChange(ctx context.Context, st store.RStore) {
	if !m.needsWatch(st) {
		return
	}
	m.watching = true

	go m.dispatchUIDsLoop(ctx, nil, st)
}

func (m *UIDMapManager) dispatchUIDsLoop(ctx context.Context, ch <-chan watch.Event, st store.RStore) {
	for {
		select {
		case <-ctx.Done():
			return
		case e, ok := <-ch:
			if !ok {
				return
			}
			if e.Type == watch.Modified {
				continue
			}
			if e.Object == nil {
				continue
			}
			if e.Object.GetObjectKind() == nil {
				continue
			}
			gvk := e.Object.GetObjectKind().GroupVersionKind()
			ke := k8s.K8sEntity{Obj: e.Object, Kind: &gvk}

			manifestName, ok := ke.Labels()[k8s.ManifestNameLabel]
			if !ok {
				continue
			}

			st.Dispatch(UIDUpdateAction{
				UID:          ke.UID(),
				EventType:    e.Type,
				ManifestName: model.ManifestName(manifestName),
				Entity:       ke})
		}
	}
}
