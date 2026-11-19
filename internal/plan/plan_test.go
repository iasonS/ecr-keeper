package plan

import (
	"reflect"
	"testing"
	"time"
)

const (
	marker = "in-use-prod-"
	first  = "sha256:aaaaaaaaaaaa1111"
	second = "sha256:bbbbbbbbbbbb2222"
)

var now = time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC)

func decide(images []Image, running map[string]bool, lastSeen map[string]time.Time) []Action {
	return ForRepository(Input{
		Marker: marker, Images: images, Running: running, LastSeen: lastSeen,
		Now: now, Grace: 14 * 24 * time.Hour,
	})
}

func TestRunningImageGetsItsMarker(t *testing.T) {
	got := decide([]Image{{Digest: first, Tags: []string{"v1"}}}, map[string]bool{first: true}, nil)
	want := []Action{{Op: Add, Digest: first, Tag: "in-use-prod-aaaaaaaaaaaa"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestMarkedRunningImageIsLeftAlone(t *testing.T) {
	images := []Image{{Digest: first, Tags: []string{"v1", "in-use-prod-aaaaaaaaaaaa"}}}
	if got := decide(images, map[string]bool{first: true}, nil); got != nil {
		t.Fatalf("got %+v, want nothing", got)
	}
}

func TestMarkerStaysDuringTheGracePeriod(t *testing.T) {
	images := []Image{{Digest: first, Tags: []string{"v1", "in-use-prod-aaaaaaaaaaaa"}}}
	lastSeen := map[string]time.Time{first: now.Add(-13 * 24 * time.Hour)}
	if got := decide(images, nil, lastSeen); got != nil {
		t.Fatalf("got %+v, want nothing", got)
	}
}

func TestMarkerIsRemovedAfterTheGracePeriod(t *testing.T) {
	images := []Image{
		{Digest: first, Tags: []string{"v1", "in-use-prod-aaaaaaaaaaaa"}},
		{Digest: second, Tags: []string{"in-use-prod-bbbbbbbbbbbb"}},
	}
	old := now.Add(-15 * 24 * time.Hour)
	got := decide(images, nil, map[string]time.Time{first: old, second: old})
	want := []Action{
		{Op: Remove, Digest: first, Tag: "in-use-prod-aaaaaaaaaaaa"},
		{Op: OnlyTag, Digest: second, Tag: "in-use-prod-bbbbbbbbbbbb"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestMarkerOfAnotherClusterIsIgnored(t *testing.T) {
	images := []Image{{Digest: first, Tags: []string{"v1", "in-use-dev-aaaaaaaaaaaa"}}}
	lastSeen := map[string]time.Time{first: now.Add(-30 * 24 * time.Hour)}
	if got := decide(images, nil, lastSeen); got != nil {
		t.Fatalf("got %+v, want nothing", got)
	}
}
