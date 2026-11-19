# ecr-keeper

Stops ECR lifecycle rules from deleting an image that still runs in a Kubernetes cluster.

Lifecycle rules count images and days; they do not know what is deployed. ecr-keeper lists
the pods of a cluster and gives every running ECR image an extra tag, `in-use-<cluster>-<digest>`.
One lifecycle rule on that tag prefix then keeps those images, because ECR never lets a
lower-priority rule expire an image that a higher-priority rule matches.

## The lifecycle rule

Put this first in the policy of each repository:

    {
      "rulePriority": 1,
      "description": "keep images that run in a cluster",
      "selection": {
        "tagStatus": "tagged",
        "tagPrefixList": ["in-use-"],
        "countType": "imageCountMoreThan",
        "countNumber": 1000
      },
      "action": { "type": "expire" }
    }

## Run

    ecr-keeper --cluster prod                 # report mode: only logs what it would tag
    ecr-keeper --cluster prod --mode apply    # writes tags
    ecr-keeper --help                         # prefix, interval, grace period

It uses the current kubeconfig context (or the pod's service account) and the default AWS
credentials. `deploy/ecr-keeper.yaml` runs it in a cluster. AWS permissions on the
repositories: `ecr:DescribeImages`, `ecr:BatchGetImage`, `ecr:PutImage`, `ecr:BatchDeleteImage`.

## What it does and does not do

- A marker stays for the grace period (14 days by default) after its image stops running, so
  a rollback still finds the image.
- It never removes an image's last tag, because ECR would delete the image with it.
- Only running images are protected. An image named in Git or in a CronJob that has not run
  within the grace period is not.
- Last-seen times are kept in memory. After a restart every grace period starts again, and
  a repository with no running image is not looked at until one of its images runs.
- One instance per cluster; several clusters can mark the same repository.

## Build

Go runs in Docker, nothing to install:

    make test     # go vet and go test
    make image    # build the image; the build also runs the tests
