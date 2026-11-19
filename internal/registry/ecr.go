// Package registry reads and writes image tags in ECR.
package registry

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/ecr/types"

	"github.com/iasonS/ecr-keeper/internal/images"
	"github.com/iasonS/ecr-keeper/internal/plan"
)

// ECR talks to ECR in whichever region a repository is in.
type ECR struct {
	Config aws.Config
}

func (e *ECR) client(repo images.Repo) *ecr.Client {
	return ecr.NewFromConfig(e.Config, func(options *ecr.Options) { options.Region = repo.Region })
}

// List returns every image of a repository with its tags.
func (e *ECR) List(ctx context.Context, repo images.Repo) ([]plan.Image, error) {
	pages := ecr.NewDescribeImagesPaginator(e.client(repo), &ecr.DescribeImagesInput{
		RegistryId:     aws.String(repo.Account),
		RepositoryName: aws.String(repo.Name),
	})

	var found []plan.Image
	for pages.HasMorePages() {
		page, err := pages.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, detail := range page.ImageDetails {
			found = append(found, plan.Image{Digest: aws.ToString(detail.ImageDigest), Tags: detail.ImageTags})
		}
	}
	return found, nil
}

// AddTag gives an image one more tag by putting its own manifest again under that tag.
func (e *ECR) AddTag(ctx context.Context, repo images.Repo, digest, tag string) error {
	client := e.client(repo)
	got, err := client.BatchGetImage(ctx, &ecr.BatchGetImageInput{
		RegistryId:     aws.String(repo.Account),
		RepositoryName: aws.String(repo.Name),
		ImageIds:       []types.ImageIdentifier{{ImageDigest: aws.String(digest)}},
	})
	if err != nil {
		return err
	}
	if len(got.Images) == 0 {
		return fmt.Errorf("image %s is not in %s", digest, repo.Name)
	}

	_, err = client.PutImage(ctx, &ecr.PutImageInput{
		RegistryId:             aws.String(repo.Account),
		RepositoryName:         aws.String(repo.Name),
		ImageManifest:          got.Images[0].ImageManifest,
		ImageManifestMediaType: got.Images[0].ImageManifestMediaType,
		ImageDigest:            aws.String(digest),
		ImageTag:               aws.String(tag),
	})
	var exists *types.ImageAlreadyExistsException
	if errors.As(err, &exists) {
		return nil
	}
	return err
}

// RemoveTag removes one tag. ECR deletes an image when its last tag goes, so the plan
// never asks for that.
func (e *ECR) RemoveTag(ctx context.Context, repo images.Repo, tag string) error {
	out, err := e.client(repo).BatchDeleteImage(ctx, &ecr.BatchDeleteImageInput{
		RegistryId:     aws.String(repo.Account),
		RepositoryName: aws.String(repo.Name),
		ImageIds:       []types.ImageIdentifier{{ImageTag: aws.String(tag)}},
	})
	if err != nil {
		return err
	}
	if len(out.Failures) > 0 {
		return fmt.Errorf("%s: %s", out.Failures[0].FailureCode, aws.ToString(out.Failures[0].FailureReason))
	}
	return nil
}
