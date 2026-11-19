GO_IMAGE := golang:1.26
GO := docker run --rm -v $(CURDIR):/src -w /src -v ecr-keeper-gocache:/go $(GO_IMAGE)

test:
	$(GO) sh -c 'go vet ./... && go test ./...'

tidy:
	$(GO) go mod tidy

image:
	docker build -t ecr-keeper:local .

.PHONY: test tidy image
