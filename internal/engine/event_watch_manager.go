package engine

import (
	"context"
	"regexp"
	"sync"

	"github.com/windmilleng/tilt/internal/model"

	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"

	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/store"
)

type uidMapValue struct {
	resourceVersion   string
	manifest          model.ManifestName
	belongsToThisTilt bool
}

type EventWatchManager struct {
	kClient  k8s.Client
	watching bool
	mapMu    sync.Mutex
	uidMap   map[k8s.UID]uidMapValue
}

func NewEventWatchManager(kClient k8s.Client) *EventWatchManager {
	return &EventWatchManager{
		kClient: kClient,
		uidMap:  make(map[k8s.UID]uidMapValue),
	}
}

func (m *EventWatchManager) needsWatch(st store.RStore) bool {
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

func (m *EventWatchManager) OnChange(ctx context.Context, st store.RStore) {
	if !m.needsWatch(st) {
		return
	}

	m.watching = true

	ch, err := m.kClient.WatchEvents(ctx)
	if err != nil {
		err = errors.Wrap(err, "Error watching k8s events\n")
		st.Dispatch(NewErrorAction(err))
		return
	}

	go m.dispatchEventsLoop(ctx, ch, st)
}

var versionRegex = regexp.MustCompile(`^v\d+$`)

func (m *EventWatchManager) getUIDMapValue(ctx context.Context, involvedObject v1.ObjectReference) uidMapValue {
	// this is annoying.
	// involvedObject.APIVersion is theoretically an APIVersion, which could be "${group}/${version}", or just "${version}".
	// if you call involvedObject.GroupVersionKind(), it will call schema.ParseGroupVersion on involvedObject.APIVersion.
	// schema.ParseGroupVersion assumes that if there are 0 slashes, the full string is just the version.
	// HOWEVER, sometimes involvedObject.APIVersion is just the group (e.g., for ReplicaSet, "extensions"), and sometimes it's just
	// the version (e.g., for Pod, "v1"). This definitely seems like a bug in k8s worth pursuing. For the moment,
	// let's use a heuristic to figure out what to do with this field.
	// Luckily, we're using DiscoveryRESTMapper, which takes care of the versions for us.

	// more info:
	// https://github.com/kubernetes/client-go/issues/308
	// https://github.com/kubernetes/kubernetes/issues/3030

	group := ""
	if !versionRegex.MatchString(involvedObject.APIVersion) {
		group = involvedObject.APIVersion
	}

	o, err := m.kClient.Get(
		group,
		"", // version is taken care of by DiscoveryRESTMapper
		involvedObject.Kind,
		involvedObject.Namespace,
		involvedObject.Name,
		involvedObject.ResourceVersion,
	)
	if err != nil {
		return uidMapValue{
			resourceVersion:   involvedObject.ResourceVersion,
			belongsToThisTilt: false,
		}
	}

	mn := model.ManifestName(o.GetLabels()[k8s.ManifestNameLabel])
	if mn == "" {
		return uidMapValue{
			resourceVersion:   involvedObject.ResourceVersion,
			belongsToThisTilt: false,
		}
	}

	return uidMapValue{
		resourceVersion:   involvedObject.ResourceVersion,
		manifest:          mn,
		belongsToThisTilt: o.GetLabels()[k8s.TiltRunIDLabel] == k8s.TiltRunID,
	}
}

func (m *EventWatchManager) dispatchEventsLoop(ctx context.Context, ch <-chan *v1.Event, st store.RStore) {
	for {
		select {
		case event, ok := <-ch:
			if !ok {
				return
			}

			uid := k8s.UID(event.InvolvedObject.UID)

			m.mapMu.Lock()
			v, ok := m.uidMap[uid]
			m.mapMu.Unlock()

			go func() {
				if !ok || v.resourceVersion != event.InvolvedObject.ResourceVersion {
					v = m.getUIDMapValue(ctx, event.InvolvedObject)
					m.mapMu.Lock()
					m.uidMap[uid] = v
					m.mapMu.Unlock()
				}

				if v.belongsToThisTilt {
					st.Dispatch(store.NewK8sEventAction(event, v.manifest))
				}
			}()

		case <-ctx.Done():
			return
		}
	}
}
