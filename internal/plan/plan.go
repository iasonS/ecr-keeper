// Package plan decides which marker tags to add and remove in one repository. It reads
// nothing and writes nothing; the keeper does that.
package plan

import (
	"slices"
	"strings"
	"time"
)

// Op is what to do with one image's marker tag.
type Op string

const (
	Add     Op = "add"
	Remove  Op = "remove"
	OnlyTag Op = "only-tag" // the marker is the image's last tag; removing it would delete the image
)

// Image is one image in the repository with all of its tags.
type Image struct {
	Digest string
	Tags   []string
}

// Action is the decision for one image.
type Action struct {
	Op     Op
	Digest string
	Tag    string
}

// Input is everything the decision needs.
type Input struct {
	Marker   string               // tag prefix of this cluster, for example "in-use-prod-"
	Images   []Image              // the repository's images
	Running  map[string]bool      // digests that run in the cluster now
	LastSeen map[string]time.Time // when each digest last ran
	Now      time.Time
	Grace    time.Duration // how long a marker stays after its image stops running
}

// MarkerTag is the tag that marks one image as in use: the marker plus the start of the
// digest, because a tag can point at only one image.
func MarkerTag(marker, digest string) string {
	hex := strings.TrimPrefix(digest, "sha256:")
	if len(hex) > 12 {
		hex = hex[:12]
	}
	return marker + hex
}

// ForRepository decides what to do for every image in one repository.
func ForRepository(in Input) []Action {
	var actions []Action
	for _, image := range in.Images {
		tag := MarkerTag(in.Marker, image.Digest)
		marked := slices.Contains(image.Tags, tag)

		if in.Running[image.Digest] {
			if !marked {
				actions = append(actions, Action{Op: Add, Digest: image.Digest, Tag: tag})
			}
			continue
		}
		if !marked {
			continue
		}

		seen, known := in.LastSeen[image.Digest]
		if !known || in.Now.Sub(seen) < in.Grace {
			continue
		}
		if len(image.Tags) == 1 {
			actions = append(actions, Action{Op: OnlyTag, Digest: image.Digest, Tag: tag})
			continue
		}
		actions = append(actions, Action{Op: Remove, Digest: image.Digest, Tag: tag})
	}
	return actions
}
