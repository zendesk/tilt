package k8s

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/runtime"

	"github.com/stretchr/testify/assert"

	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/testutils/output"
)

func TestK8sClient_WatchEverything(t *testing.T) {
	tf := newWatchEverythingTestFixture(t)

	tf.fakeClient.AddReactor("get", "group", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		handled = true
		err = nil
		ret = &metav1.APIGroupList{
			Groups: []metav1.APIGroup{
				{
					Name: "group1",
				},
			},
		}
		return
	})

	groups, _, _ := tf.fakeClient.Discovery().ServerGroupsAndResources()

	fmt.Println(groups)

	fmt.Println(tf.fakeClient.Fake.Actions())
}

func TestK8sClient_WatchEverythingNoResources(t *testing.T) {
	tf := newWatchEverythingTestFixture(t)
	defer tf.TearDown()

	// NOTE(dmiller): because we don't add any resources in to the
	// fake clientset that we set up in `newWatchTestFixture` `ServerGroupsAndResources()`
	// returns an empty list, which triggers the following error
	tf.watchEverythingExpectError("Unable to watch any resources: do you have sufficient permissions to watch resources?")
}

type watchEverythingTestFixture struct {
	t                 *testing.T
	kCli              K8sClient
	w                 *watch.FakeWatcher
	watchRestrictions k8stesting.WatchRestrictions
	ctx               context.Context
	watchErr          error
	nsRestriction     Namespace
	cancel            context.CancelFunc
	fakeClient        *fake.Clientset
}

func newWatchEverythingTestFixture(t *testing.T) *watchEverythingTestFixture {
	ret := &watchEverythingTestFixture{t: t}

	c := fake.NewSimpleClientset()

	ret.ctx, ret.cancel = context.WithCancel(output.CtxForTest())

	ret.w = watch.NewFakeWithChanSize(10, false)

	wr := func(action k8stesting.Action) (handled bool, wi watch.Interface, err error) {
		wa := action.(k8stesting.WatchAction)
		nsRestriction := ret.nsRestriction
		if !nsRestriction.Empty() && wa.GetNamespace() != nsRestriction.String() {
			return true, nil, &apiErrors.StatusError{
				ErrStatus: metav1.Status{Code: http.StatusForbidden},
			}
		}
		ret.watchRestrictions = wa.GetWatchRestrictions()
		if ret.watchErr != nil {
			return true, nil, ret.watchErr
		}
		return true, ret.w, nil
	}

	c.Fake.PrependWatchReactor("*", wr)

	ret.fakeClient = c

	ret.kCli = K8sClient{
		env:           EnvUnknown,
		kubectlRunner: nil,
		core:          c.CoreV1(), // TODO set
		restConfig:    nil,
		portForwarder: nil,
		clientSet:     c,
	}

	return ret
}

func (tf *watchEverythingTestFixture) TearDown() {
	tf.cancel()
}

func (tf *watchEverythingTestFixture) watchEverythingExpectError(expectedErr string) {
	_, err := tf.kCli.WatchEverything(tf.ctx, []model.LabelPair{})
	assert.EqualError(tf.t, err, expectedErr)
}

func (tf *watchTestFixture) runWatchEverything(input, expectedOutput []runtime.Object) {
	for _, o := range input {
		tf.w.Add(o)
	}

	tf.w.Stop()

	ch, err := tf.kCli.WatchEverything(tf.ctx, []model.LabelPair{})
	if !assert.NoError(tf.t, err) {
		return
	}

	var observedObjects []runtime.Object

	timeout := time.After(500 * time.Millisecond)
	done := false
	for !done {
		select {
		case event, ok := <-ch:
			if !ok {
				done = true
			} else {
				observedObjects = append(observedObjects, event.Object)
			}
		case <-timeout:
			tf.t.Fatalf("test timed out\nExpected objects: %v\nObserved objects: %v\n", expectedOutput, observedObjects)
		case <-time.After(10 * time.Millisecond):
			// if we haven't seen any events for 10ms, assume we're done
			// ideally we'd always be exiting from ch closing, but it's not currently clear how to do that via informer
			done = true
		}
	}

	// TODO(matt) - using ElementsMatch instead of Equal because, for some reason, events do not always come out in the
	// same order we feed them in. I'm punting on figuring out why for now.
	assert.ElementsMatch(tf.t, expectedOutput, observedObjects)
}
