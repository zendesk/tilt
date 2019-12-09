package engine

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/windmilleng/wmclient/pkg/dirs"

	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/internal/testutils/tempdir"
)

func TestNoTelScriptTimeIsUpShouldDeleteFile(t *testing.T) {
	f := newTCFixture(t)
	tc := f.newTelemetryController()
	ctx := context.Background()

	f.writeToAnalyticsFile("hello world")
	st, _ := store.NewStoreForTesting()
	tc.OnChange(ctx, st)

	f.assertTelemetryFileIsEmpty()
}

type tcFixture struct {
	t *testing.T
	temp *tempdir.TempDirFixture
	dir *dirs.WindmillDir
	lock *sync.Mutex
	clock fakeClock
}

func newTCFixture(t *testing.T) *tcFixture {
	temp := tempdir.NewTempDirFixture(t)
	dir := dirs.NewWindmillDirAt(temp.Path())
	lock := &sync.Mutex{}

	return &tcFixture{
		t: t,
		temp: temp,
		dir: dir,
		lock: lock,
		clock: fakeClock{now: time.Unix(1551202573, 0)},
	}
}

func (tcf *tcFixture) newTelemetryController() *TelemetryController {
	return NewTelemetryController(tcf.lock, tcf.clock, tcf.dir)
}

func (tcf *tcFixture) writeToAnalyticsFile(contents string) {
	err := tcf.dir.WriteFile(AnalyticsFilename, contents)
	if err != nil {
		tcf.t.Fatal(err)
	}
}

func (tcf *tcFixture) assertTelemetryFileIsEmpty() {
	fileContents, err := tcf.dir.ReadFile(AnalyticsFilename)
	if err != nil {
		tcf.t.Fatal(err)
	}

	assert.Empty(tcf.t, fileContents)
}
