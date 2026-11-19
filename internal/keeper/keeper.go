// Package keeper marks the ECR images that run in a cluster, so a lifecycle rule can keep them.
package keeper

import (
	"context"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/iasonS/ecr-keeper/internal/images"
	"github.com/iasonS/ecr-keeper/internal/plan"
)

// Pods lists the ECR images that pods in the cluster run now.
type Pods interface {
	Running(ctx context.Context) ([]images.Ref, error)
}

// Registry reads and writes tags in ECR.
type Registry interface {
	List(ctx context.Context, repo images.Repo) ([]plan.Image, error)
	AddTag(ctx context.Context, repo images.Repo, digest, tag string) error
	RemoveTag(ctx context.Context, repo images.Repo, tag string) error
}

// Keeper holds what one cluster's keeper needs between rounds.
type Keeper struct {
	Pods     Pods
	Registry Registry
	Marker   string        // tag prefix of this cluster, for example "in-use-prod-"
	Grace    time.Duration // how long a marker stays after its image stops running
	Apply    bool          // false only logs what would change
	Now      func() time.Time

	// Kept in memory only. After a restart the grace period of every stopped image starts
	// again, and a repository with no running image is not looked at until one runs.
	lastSeen map[images.Repo]map[string]time.Time
}

// RunOnce does one round: read the running images, then fix the markers of every
// repository seen so far.
func (k *Keeper) RunOnce(ctx context.Context) error {
	refs, err := k.Pods.Running(ctx)
	if err != nil {
		return err
	}

	now := k.Now()
	running := map[images.Repo]map[string]bool{}
	for _, ref := range refs {
		if running[ref.Repo] == nil {
			running[ref.Repo] = map[string]bool{}
		}
		running[ref.Repo][ref.Digest] = true
		k.seen(ref.Repo, ref.Digest, now)
	}

	for _, repo := range k.repos() {
		k.keep(ctx, repo, running[repo], now)
	}
	return nil
}

func (k *Keeper) keep(ctx context.Context, repo images.Repo, running map[string]bool, now time.Time) {
	log := slog.With("repository", repo.Name, "region", repo.Region)

	found, err := k.Registry.List(ctx, repo)
	if err != nil {
		log.Error("could not list images", "error", err)
		return
	}
	k.startGraceOfUnknown(repo, found, running, now)

	actions := plan.ForRepository(plan.Input{
		Marker: k.Marker, Images: found, Running: running,
		LastSeen: k.lastSeen[repo], Now: now, Grace: k.Grace,
	})
	for _, action := range actions {
		log.Info("marker", "op", action.Op, "tag", action.Tag, "digest", action.Digest, "applied", k.Apply)
		if !k.Apply {
			continue
		}
		if err := k.do(ctx, repo, action); err != nil {
			log.Error("could not change marker", "op", action.Op, "tag", action.Tag, "error", err)
		}
	}
}

func (k *Keeper) do(ctx context.Context, repo images.Repo, action plan.Action) error {
	switch action.Op {
	case plan.Add:
		return k.Registry.AddTag(ctx, repo, action.Digest, action.Tag)
	case plan.Remove:
		if err := k.Registry.RemoveTag(ctx, repo, action.Tag); err != nil {
			return err
		}
		delete(k.lastSeen[repo], action.Digest)
	}
	return nil
}

// startGraceOfUnknown gives a marked image that this process never saw running a grace
// period that starts now.
func (k *Keeper) startGraceOfUnknown(repo images.Repo, found []plan.Image, running map[string]bool, now time.Time) {
	for _, image := range found {
		if running[image.Digest] || !hasMarker(image, k.Marker) {
			continue
		}
		if _, known := k.lastSeen[repo][image.Digest]; !known {
			k.seen(repo, image.Digest, now)
		}
	}
}

func (k *Keeper) seen(repo images.Repo, digest string, at time.Time) {
	if k.lastSeen == nil {
		k.lastSeen = map[images.Repo]map[string]time.Time{}
	}
	if k.lastSeen[repo] == nil {
		k.lastSeen[repo] = map[string]time.Time{}
	}
	k.lastSeen[repo][digest] = at
}

func (k *Keeper) repos() []images.Repo {
	repos := make([]images.Repo, 0, len(k.lastSeen))
	for repo := range k.lastSeen {
		repos = append(repos, repo)
	}
	slices.SortFunc(repos, func(a, b images.Repo) int {
		return strings.Compare(a.Account+a.Region+a.Name, b.Account+b.Region+b.Name)
	})
	return repos
}

func hasMarker(image plan.Image, marker string) bool {
	return slices.Contains(image.Tags, plan.MarkerTag(marker, image.Digest))
}
