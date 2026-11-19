package keeper

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/iasonS/ecr-keeper/internal/images"
	"github.com/iasonS/ecr-keeper/internal/plan"
)

const digest = "sha256:aaaaaaaaaaaa1111"

var repo = images.Repo{Account: "123456789012", Region: "eu-west-1", Name: "api"}

type fakePods struct{ refs []images.Ref }

func (f *fakePods) Running(context.Context) ([]images.Ref, error) { return f.refs, nil }

type fakeRegistry struct {
	tags  map[string][]string // by digest
	calls []string
}

func (f *fakeRegistry) List(context.Context, images.Repo) ([]plan.Image, error) {
	var found []plan.Image
	for digest, tags := range f.tags {
		found = append(found, plan.Image{Digest: digest, Tags: tags})
	}
	return found, nil
}

func (f *fakeRegistry) AddTag(_ context.Context, _ images.Repo, digest, tag string) error {
	f.calls = append(f.calls, "add "+tag)
	f.tags[digest] = append(f.tags[digest], tag)
	return nil
}

func (f *fakeRegistry) RemoveTag(_ context.Context, _ images.Repo, tag string) error {
	f.calls = append(f.calls, "remove "+tag)
	return nil
}

func TestMarkerLivesWithTheImage(t *testing.T) {
	now := time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC)
	pods := &fakePods{refs: []images.Ref{{Repo: repo, Digest: digest}}}
	registry := &fakeRegistry{tags: map[string][]string{digest: {"v1"}}}
	keeper := &Keeper{
		Pods: pods, Registry: registry, Marker: "in-use-prod-", Grace: 24 * time.Hour, Apply: true,
		Now: func() time.Time { return now },
	}
	round := func() {
		t.Helper()
		if err := keeper.RunOnce(context.Background()); err != nil {
			t.Fatal(err)
		}
	}

	round() // running: marked
	round() // still running: nothing
	pods.refs = nil
	now = now.Add(23 * time.Hour)
	round() // stopped, inside the grace period: nothing
	now = now.Add(2 * time.Hour)
	round() // stopped for longer than the grace period: unmarked

	want := []string{"add in-use-prod-aaaaaaaaaaaa", "remove in-use-prod-aaaaaaaaaaaa"}
	if !reflect.DeepEqual(registry.calls, want) {
		t.Fatalf("got %v, want %v", registry.calls, want)
	}
}

func TestReportModeWritesNothing(t *testing.T) {
	registry := &fakeRegistry{tags: map[string][]string{digest: {"v1"}}}
	keeper := &Keeper{
		Pods: &fakePods{refs: []images.Ref{{Repo: repo, Digest: digest}}}, Registry: registry,
		Marker: "in-use-prod-", Grace: time.Hour, Now: time.Now,
	}
	if err := keeper.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if registry.calls != nil {
		t.Fatalf("report mode wrote: %v", registry.calls)
	}
}
