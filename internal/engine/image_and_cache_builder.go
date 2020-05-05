package engine

import (
	"context"
	"fmt"

	"github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/engine/buildcontrol"
	"github.com/windmilleng/tilt/internal/ignore"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/pkg/model"
)

// TODO(nick): Rename these builders. ImageBuilder should really be called DockerBuilder,
// and this struct should be called ImageBuilder.
type imageAndCacheBuilder struct {
	ib         build.ImageBuilder
	custb      build.CustomBuilder
	updateMode buildcontrol.UpdateMode

	cache map[container.RefSelector]CacheEntry
}

func NewImageAndCacheBuilder(ib build.ImageBuilder, custb build.CustomBuilder, updateMode buildcontrol.UpdateMode) *imageAndCacheBuilder {
	return &imageAndCacheBuilder{
		ib:         ib,
		custb:      custb,
		updateMode: updateMode,
	}
}

func (icb *imageAndCacheBuilder) Build(ctx context.Context, iTarget model.ImageTarget, state store.BuildState,
	ps *build.PipelineState) (refs container.TaggedRefs, err error) {
	userFacingRefName := container.FamiliarString(iTarget.Refs.ConfigurationRef)

	if refs, err, ok := icb.lookupCache(ctx, iTarget, state, ps); ok {
		return refs, err
	}

	defer func() {
		icb.putCache(ctx, iTarget, state, ps, refs, err)
	}()

	switch bd := iTarget.BuildDetails.(type) {
	case model.DockerBuild:
		ps.StartPipelineStep(ctx, "Building Dockerfile: [%s]", userFacingRefName)
		defer ps.EndPipelineStep(ctx)

		refs, err = icb.ib.BuildImage(ctx, ps, iTarget.Refs, bd,
			ignore.CreateBuildContextFilter(iTarget))

		if err != nil {
			return container.TaggedRefs{}, err
		}
	case model.CustomBuild:
		ps.StartPipelineStep(ctx, "Building Custom Build: [%s]", userFacingRefName)
		defer ps.EndPipelineStep(ctx)
		refs, err = icb.custb.Build(ctx, iTarget.Refs, bd)
		if err != nil {
			return container.TaggedRefs{}, err
		}
	default:
		// Theoretically this should never trip b/c we `validate` the manifest beforehand...?
		// If we get here, something is very wrong.
		return container.TaggedRefs{}, fmt.Errorf("image %q has no valid buildDetails (neither "+
			"DockerBuild nor CustomBuild)", iTarget.Refs.ConfigurationRef)
	}

	return refs, nil
}

func (icb *imageAndCacheBuilder) checkCache(ctx context.Context, iTarget model.ImageTarget, state store.BuildState, ps *build.PipelineState) (refs container.TaggedRefs, err error, ok bool) {
	if state.ImageBuildTriggered {
		// explicit build requested
		return nil, nil, false
	}

	entry := icb.cache[iTarget.Refs.ConfigurationRef]
	if entry.IsZero() {
		// no cached entry
		return nil, nil, false
	}

	if state.MostRecentMod().After(entry.LastChange) {
		// there was a file modified since we built this
		return nil, nil, false
	}

	return entry.Refs, entry.Err, true
}

func (icb *imageAndCacheBuilder) putCache(ctx context.Context, iTarget model.ImageTarget, state store.BuildState, ps *build.PipelineState, refs container.TaggedRefs, err error) {
	icb.cache[iTarget.Refs.ConfigurationRef] = CacheEntry{
		ITarget:    iTarget,
		LastChange: state.MostRecentMod(),
		Refs:       refs,
		Err:        err,
	}
}

type CacheEntry struct {
	ITarget    model.ImageTarget
	LastChange time.Time

	Refs container.TaggedRefs
	Err  error
}
