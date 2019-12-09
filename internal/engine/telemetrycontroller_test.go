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
	temp := tempdir.NewTempDirFixture(t)
	t1 := fakeClock{now: time.Unix(1551202573, 0)}
	dir := dirs.NewWindmillDirAt(temp.Path())
	err := dir.WriteFile("analytics", "hello world")
	if err != nil {
		t.Fatal(err)
	}
	lock := &sync.Mutex{}
	st, _ := store.NewStoreForTesting()
	ctx := context.Background()

	tc := NewTelemetryController(lock, t1, dir)

	tc.OnChange(ctx, st)

	fileContents, err := dir.ReadFile("analytics")
	if err != nil {
		t.Fatal(err)
	}

	assert.Empty(t, fileContents)
}