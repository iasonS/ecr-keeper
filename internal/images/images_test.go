package images

import "testing"

func TestParse(t *testing.T) {
	const digest = "sha256:3f2a9c1b4d5e6f708192a3b4c5d6e7f8091a2b3c4d5e6f708192a3b4c5d6e7f8"
	const registry = "123456789012.dkr.ecr.eu-west-1.amazonaws.com"

	ref, found := Parse(registry+"/team/api:v1", registry+"/team/api@"+digest)
	if !found {
		t.Fatal("an ECR image with a digest was not recognised")
	}
	want := Ref{Repo: Repo{Account: "123456789012", Region: "eu-west-1", Name: "team/api"}, Digest: digest}
	if ref != want {
		t.Fatalf("got %+v, want %+v", ref, want)
	}

	if _, found := Parse("nginx:1.27", "docker.io/library/nginx@"+digest); found {
		t.Fatal("an image outside ECR was recognised")
	}
	if _, found := Parse(registry+"/team/api:v1", ""); found {
		t.Fatal("an image without a reported digest was recognised")
	}
}
