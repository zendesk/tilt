package buildcontrol

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/docker/distribution/reference"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/tilt-dev/tilt/internal/analytics"
	"github.com/tilt-dev/tilt/internal/build"
	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/dockerfile"
	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

var _ BuildAndDeployer = &ImageBuildAndDeployer{}

type KINDLoader interface {
	LoadToKIND(ctx context.Context, ref reference.NamedTagged) error
}

type cmdKINDLoader struct {
	env         k8s.Env
	clusterName k8s.ClusterName
}

func (kl *cmdKINDLoader) LoadToKIND(ctx context.Context, ref reference.NamedTagged) error {
	// In Kind5, --name specifies the name of the cluster in the kubeconfig.
	// In Kind6, the -name parameter is prefixed with 'kind-' before being written to/read from the kubeconfig
	kindName := string(kl.clusterName)
	if kl.env == k8s.EnvKIND6 {
		kindName = strings.TrimPrefix(kindName, "kind-")
	}

	cmd := exec.CommandContext(ctx, "kind", "load", "docker-image", ref.String(), "--name", kindName)
	w := logger.NewMutexWriter(logger.Get(ctx).Writer(logger.InfoLvl))
	cmd.Stdout = w
	cmd.Stderr = w

	return cmd.Run()
}

func NewKINDLoader(env k8s.Env, clusterName k8s.ClusterName) KINDLoader {
	return &cmdKINDLoader{
		env:         env,
		clusterName: clusterName,
	}
}

type ImageBuildAndDeployer struct {
	db          build.DockerBuilder
	ib          *ImageBuilder
	k8sClient   k8s.Client
	env         k8s.Env
	kubeContext k8s.KubeContext
	analytics   *analytics.TiltAnalytics
	clock       build.Clock
	kl          KINDLoader
}

func NewImageBuildAndDeployer(
	db build.DockerBuilder,
	customBuilder build.CustomBuilder,
	k8sClient k8s.Client,
	env k8s.Env,
	kubeContext k8s.KubeContext,
	analytics *analytics.TiltAnalytics,
	updMode UpdateMode,
	c build.Clock,
	kl KINDLoader,
) *ImageBuildAndDeployer {
	return &ImageBuildAndDeployer{
		db:          db,
		ib:          NewImageBuilder(db, customBuilder, updMode),
		k8sClient:   k8sClient,
		env:         env,
		kubeContext: kubeContext,
		analytics:   analytics,
		clock:       c,
		kl:          kl,
	}
}

func (ibd *ImageBuildAndDeployer) BuildAndDeploy(ctx context.Context, st store.RStore, specs []model.TargetSpec, stateSet store.BuildStateSet) (resultSet store.BuildResultSet, err error) {
	iTargets, kTargets := extractImageAndK8sTargets(specs)
	if len(kTargets) != 1 {
		return store.BuildResultSet{}, SilentRedirectToNextBuilderf("ImageBuildAndDeployer does not support these specs")
	}

	kTarget := kTargets[0]

	startTime := time.Now()
	defer func() {
		ibd.analytics.Timer("build.image", time.Since(startTime), map[string]string{
			"hasError": fmt.Sprintf("%t", err != nil),
		})
	}()

	q, err := NewImageTargetQueue(ctx, iTargets, stateSet, ibd.ib.CanReuseRef)
	if err != nil {
		return store.BuildResultSet{}, err
	}

	// each image target has two stages: one for build, and one for push
	numStages := q.CountBuilds()*2 + 1

	reused := q.ReusedResults()
	hasReusedStep := len(reused) > 0
	if hasReusedStep {
		numStages++
	}

	hasDeleteStep := stateSet.FullBuildTriggered()
	if hasDeleteStep {
		numStages++
	}

	ps := build.NewPipelineState(ctx, numStages, ibd.clock)
	defer func() { ps.End(ctx, err) }()

	if hasDeleteStep {
		ps.StartPipelineStep(ctx, "Force update")
		err = ibd.delete(ps.AttachLogger(ctx), kTarget)
		if err != nil {
			return store.BuildResultSet{}, WrapDontFallBackError(err)
		}
		ps.EndPipelineStep(ctx)
	}

	if hasReusedStep {
		ps.StartPipelineStep(ctx, "Loading cached images")
		for _, result := range reused {
			ref := store.LocalImageRefFromBuildResult(result)
			ps.Printf(ctx, "- %s", container.FamiliarString(ref))
		}
		ps.EndPipelineStep(ctx)
	}

	iTargetMap := model.ImageTargetsByID(iTargets)
	err = q.RunBuilds(func(target model.TargetSpec, depResults []store.BuildResult) (store.BuildResult, error) {
		iTarget, ok := target.(model.ImageTarget)
		if !ok {
			return nil, fmt.Errorf("Not an image target: %T", target)
		}

		iTarget, err := InjectImageDependencies(iTarget, iTargetMap, depResults)
		if err != nil {
			return nil, err
		}

		refs, err := ibd.ib.Build(ctx, iTarget, ps)
		if err != nil {
			return nil, err
		}

		err = ibd.push(ctx, refs.LocalRef, ps, iTarget, kTarget)
		if err != nil {
			return nil, err
		}

		return store.NewImageBuildResult(iTarget.ID(), refs.LocalRef, refs.ClusterRef), nil
	})

	newResults := q.NewResults()
	if err != nil {
		return newResults, WrapDontFallBackError(err)
	}

	startDeployTime := time.Now()

	// (If we pass an empty list of refs here (as we will do if only deploying
	// yaml), we just don't inject any image refs into the yaml, nbd.
	k8sResult, err := ibd.deploy(ctx, st, ps, iTargetMap, kTarget, q.AllResults())
	reportK8sDeployMetrics(ctx, kTarget, time.Since(startDeployTime), err != nil)
	if err != nil {
		return newResults, WrapDontFallBackError(err)
	}
	newResults[kTarget.ID()] = k8sResult
	return newResults, nil
}

func (ibd *ImageBuildAndDeployer) push(ctx context.Context, ref reference.NamedTagged, ps *build.PipelineState, iTarget model.ImageTarget, kTarget model.K8sTarget) error {
	ps.StartPipelineStep(ctx, "Pushing %s", container.FamiliarString(ref))
	defer ps.EndPipelineStep(ctx)

	cbSkip := false
	if iTarget.IsCustomBuild() {
		cbSkip = iTarget.CustomBuildInfo().SkipsPush()
	}

	// We can also skip the push of the image if it isn't used
	// in any k8s resources! (e.g., it's consumed by another image).

	if cbSkip {
		ps.Printf(ctx, "Skipping push: custom_build() configured to handle push itself")
		return nil
	} else if !IsImageDeployedToK8s(iTarget, kTarget) {
		ps.Printf(ctx, "Skipping push: base image does not need deploy")
		return nil
	} else if ibd.db.WillBuildToKubeContext(ibd.kubeContext) {
		ps.Printf(ctx, "Skipping push: building on cluster's container runtime")
		return nil
	}

	var err error
	if ibd.shouldUseKINDLoad(ctx, iTarget) {
		ps.Printf(ctx, "Loading image to KIND")
		err := ibd.kl.LoadToKIND(ps.AttachLogger(ctx), ref)
		if err != nil {
			return fmt.Errorf("Error loading image to KIND: %v", err)
		}
	} else {
		ps.Printf(ctx, "Pushing with Docker client")
		err = ibd.db.PushImage(ps.AttachLogger(ctx), ref)
		if err != nil {
			return err
		}
	}

	return nil
}

func (ibd *ImageBuildAndDeployer) shouldUseKINDLoad(ctx context.Context, iTarg model.ImageTarget) bool {
	isKIND := ibd.env == k8s.EnvKIND5 || ibd.env == k8s.EnvKIND6
	if !isKIND {
		return false
	}

	// if we're using KIND and the image has a separate ref by which it's referred to
	// in the cluster, that implies that we have a local registry in place, and should
	// push to that instead of using KIND load.
	if iTarg.HasDistinctClusterRef() {
		return false
	}

	registry := ibd.k8sClient.LocalRegistry(ctx)
	if !registry.Empty() {
		return false
	}

	return true
}

// Returns: the entities deployed and the namespace of the pod with the given image name/tag.
func (ibd *ImageBuildAndDeployer) deploy(ctx context.Context, st store.RStore, ps *build.PipelineState,
	iTargetMap map[model.TargetID]model.ImageTarget, kTarget model.K8sTarget, results store.BuildResultSet) (store.BuildResult, error) {
	ps.StartPipelineStep(ctx, "Deploying")
	defer ps.EndPipelineStep(ctx)

	ps.StartBuildStep(ctx, "Injecting images into Kubernetes YAML")

	// Create API objects.
	spec := kTarget.KubernetesApplySpec
	imageMaps := make(map[string]*v1alpha1.ImageMap)
	for _, imageMapName := range spec.ImageMaps {
		depID := model.TargetID{
			Type: model.TargetTypeImage,
			Name: model.TargetName(imageMapName),
		}

		iTarget, ok := iTargetMap[depID]
		if !ok {
			return nil, fmt.Errorf("Internal error: missing image target for dependency ID: %s", depID)
		}

		ref := store.ClusterImageRefFromBuildResult(results[depID])
		if ref == nil {
			return nil, fmt.Errorf("Internal error: missing image build result for dependency ID: %s", depID)
		}

		name := string(depID.Name)
		imageMaps[name] = &v1alpha1.ImageMap{
			ObjectMeta: metav1.ObjectMeta{Name: name},
			Spec:       iTarget.ImageMapSpec,
			Status: v1alpha1.ImageMapStatus{
				Image: ref.String(),
			},
		}
	}

	newK8sEntities, err := ibd.createEntitiesToDeploy(ctx, imageMaps, spec)
	if err != nil {
		return nil, err
	}

	ctx = ibd.indentLogger(ctx)
	l := logger.Get(ctx)

	l.Infof("Applying via kubectl:")
	for _, displayName := range kTarget.DisplayNames {
		l.Infof("→ %s", displayName)
	}

	timeout := kTarget.Timeout.Duration
	if timeout == 0 {
		timeout = v1alpha1.KubernetesApplyTimeoutDefault
	}

	deployed, err := ibd.k8sClient.Upsert(ctx, newK8sEntities, timeout)
	if err != nil {
		return nil, err
	}

	podTemplateSpecHashes := []k8s.PodTemplateSpecHash{}
	for _, entity := range deployed {
		if entity.UID() == "" {
			return nil, fmt.Errorf("Entity not deployed correctly: %v", entity)
		}
		hs, err := k8s.ReadPodTemplateSpecHashes(entity)
		if err != nil {
			return nil, errors.Wrap(err, "reading pod template spec hashes")
		}
		podTemplateSpecHashes = append(podTemplateSpecHashes, hs...)
	}

	return store.NewK8sDeployResult(kTarget.ID(), podTemplateSpecHashes, deployed), nil
}

func (ibd *ImageBuildAndDeployer) indentLogger(ctx context.Context) context.Context {
	l := logger.Get(ctx)
	newL := logger.NewPrefixedLogger(logger.Blue(l).Sprint("     "), l)
	return logger.WithLogger(ctx, newL)
}

// Delete all the resources in the Kubernetes target, to ensure that they restart when
// we re-apply them.
//
// Namespaces are not deleted by default. Similar to `tilt down`, deleting namespaces
// is likely to be more destructive than most users want from this operation.
func (ibd *ImageBuildAndDeployer) delete(ctx context.Context, k8sTarget model.K8sTarget) error {
	entities, err := k8s.ParseYAMLFromString(k8sTarget.YAML)
	if err != nil {
		return err
	}

	entities, _, err = k8s.Filter(entities, func(e k8s.K8sEntity) (b bool, err error) {
		return e.GVK() != schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Namespace"}, nil
	})
	if err != nil {
		return err
	}

	entities = k8s.ReverseSortedEntities(entities)

	return ibd.k8sClient.Delete(ctx, entities)
}

func (ibd *ImageBuildAndDeployer) createEntitiesToDeploy(ctx context.Context,
	imageMaps map[string]*v1alpha1.ImageMap,
	spec v1alpha1.KubernetesApplySpec) ([]k8s.K8sEntity, error) {
	newK8sEntities := []k8s.K8sEntity{}

	entities, err := k8s.ParseYAMLFromString(spec.YAML)
	if err != nil {
		return nil, err
	}

	locators, err := k8s.ParseImageLocators(spec.ImageLocators)
	if err != nil {
		return nil, err
	}

	imageMapNames := spec.ImageMaps
	injectedImageMaps := map[string]bool{}
	for _, e := range entities {
		e, err = k8s.InjectLabels(e, []model.LabelPair{
			k8s.TiltManagedByLabel(),
		})
		if err != nil {
			return nil, errors.Wrap(err, "deploy")
		}

		// If we're redeploying these workloads in response to image
		// changes, we make sure image pull policy isn't set to "Always".
		// Frequent applies don't work well with this setting, and makes things
		// slower. See discussion:
		// https://github.com/tilt-dev/tilt/issues/3209
		if len(imageMaps) > 0 {
			e, err = k8s.InjectImagePullPolicy(e, v1.PullIfNotPresent)
			if err != nil {
				return nil, err
			}
		}

		if len(imageMaps) > 0 {
			// StatefulSet pods should be managed in parallel when we're doing iterative
			// development. See discussion:
			// https://github.com/tilt-dev/tilt/issues/1962
			// https://github.com/tilt-dev/tilt/issues/3906
			e = k8s.InjectParallelPodManagementPolicy(e)
		}

		// When working with a local k8s cluster, we set the pull policy to Never,
		// to ensure that k8s fails hard if the image is missing from docker.
		policy := v1.PullIfNotPresent
		if ibd.db.WillBuildToKubeContext(ibd.kubeContext) {
			policy = v1.PullNever
		}

		for _, imageMapName := range imageMapNames {
			imageMap := imageMaps[imageMapName]
			imageMapSpec := imageMap.Spec
			selector, err := container.SelectorFromImageMap(imageMapSpec)
			if err != nil {
				return nil, err
			}
			matchInEnvVars := imageMapSpec.MatchInEnvVars

			ref, err := reference.ParseNamed(imageMap.Status.Image)
			if err != nil {
				return nil, fmt.Errorf("parsing image map status: %v", err)
			}

			var replaced bool
			e, replaced, err = k8s.InjectImageDigest(e, selector, ref, locators, matchInEnvVars, policy)
			if err != nil {
				return nil, err
			}
			if replaced {
				injectedImageMaps[imageMapName] = true

				if imageMapSpec.OverrideCommand != nil || imageMapSpec.OverrideArgs != nil {
					e, err = k8s.InjectCommandAndArgs(e, ref, imageMapSpec.OverrideCommand, imageMapSpec.OverrideArgs)
					if err != nil {
						return nil, err
					}
				}
			}
		}

		// This needs to be after all the other injections, to ensure the hash includes the Tilt-generated
		// image tag, etc
		e, err := k8s.InjectPodTemplateSpecHashes(e)
		if err != nil {
			return nil, errors.Wrap(err, "injecting pod template hash")
		}

		newK8sEntities = append(newK8sEntities, e)
	}

	for _, name := range imageMapNames {
		if !injectedImageMaps[name] {
			return nil, fmt.Errorf("Docker image missing from yaml: %s", name)
		}
	}

	return newK8sEntities, nil
}

// Create a new ImageTarget with the Dockerfiles rewritten with the injected images.
func InjectImageDependencies(iTarget model.ImageTarget, iTargetMap map[model.TargetID]model.ImageTarget, deps []store.BuildResult) (model.ImageTarget, error) {
	if len(deps) == 0 {
		return iTarget, nil
	}

	df := dockerfile.Dockerfile("")
	switch bd := iTarget.BuildDetails.(type) {
	case model.DockerBuild:
		df = dockerfile.Dockerfile(bd.Dockerfile)
	default:
		return model.ImageTarget{}, fmt.Errorf("image %q has no valid buildDetails", iTarget.Refs.ConfigurationRef)
	}

	ast, err := dockerfile.ParseAST(df)
	if err != nil {
		return model.ImageTarget{}, errors.Wrap(err, "injectImageDependencies")
	}

	for _, dep := range deps {
		image := store.LocalImageRefFromBuildResult(dep)
		if image == nil {
			return model.ImageTarget{}, fmt.Errorf("Internal error: image is nil")
		}
		id := dep.TargetID()
		modified, err := ast.InjectImageDigest(iTargetMap[id].Refs.ConfigurationRef, image)
		if err != nil {
			return model.ImageTarget{}, errors.Wrap(err, "injectImageDependencies")
		} else if !modified {
			return model.ImageTarget{}, fmt.Errorf("Could not inject image %q into Dockerfile of image %q", image, iTarget.Refs.ConfigurationRef)
		}
	}

	newDf, err := ast.Print()
	if err != nil {
		return model.ImageTarget{}, errors.Wrap(err, "injectImageDependencies")
	}

	bd := iTarget.DockerBuildInfo()
	bd.Dockerfile = newDf.String()
	iTarget = iTarget.WithBuildDetails(bd)

	return iTarget, nil
}
