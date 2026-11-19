// Package images reads ECR image references out of what a pod reports.
package images

import (
	"regexp"
	"strings"
)

// Repo is one ECR repository.
type Repo struct {
	Account string
	Region  string
	Name    string
}

// Ref is one image in an ECR repository, by digest.
type Ref struct {
	Repo   Repo
	Digest string
}

var ecrImage = regexp.MustCompile(`^(\d{12})\.dkr\.ecr\.([a-z0-9-]+)\.amazonaws\.com(?:\.cn)?/([^@:]+)`)

// Parse turns a container's image name and image ID into a Ref. It reports false for an
// image that is not in ECR or whose digest the node has not reported yet.
func Parse(image, imageID string) (Ref, bool) {
	match := ecrImage.FindStringSubmatch(image)
	if match == nil {
		return Ref{}, false
	}

	at := strings.LastIndex(imageID, "@sha256:")
	if at < 0 {
		return Ref{}, false
	}

	repo := Repo{Account: match[1], Region: match[2], Name: match[3]}
	return Ref{Repo: repo, Digest: imageID[at+1:]}, true
}
