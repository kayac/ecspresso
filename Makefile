GIT_VER ?= $(shell git describe --tags | sed -e 's/-/+/')
DATE := $(shell date +%Y-%m-%dT%H:%M:%S%z)

.PHONY: test binary install clean

cmd/ecspresso/ecspresso: *.go cmd/ecspresso/*.go go.* */*.go
	cd cmd/ecspresso && go build -tags "no_gcs,no_azurerm" -ldflags "-s -w -X github.com/kayac/ecspresso/v3.Version=${GIT_VER}" -trimpath

install: cmd/ecspresso/ecspresso
	install cmd/ecspresso/ecspresso `go env GOPATH`/bin/ecspresso

test:
	go test -race ./...

packages:
	goreleaser build --skip-validate --clean

packages-snapshot:
	goreleaser build --skip-validate --clean --snapshot

clean:
	rm -f cmd/ecspresso/ecspresso
	rm -rf dist/*

ci-test:
	$(MAKE) install
	cd tests/ci && PATH=${GOPATH}/bin:$PATH $(MAKE) test

orb/publish:
	circleci orb validate orb.yml
	circleci orb publish orb.yml $(ORB_NAMESPACE)/ecspresso@dev:latest

orb/promote:
	circleci orb publish promote $(ORB_NAMESPACE)/ecspresso@dev:latest patch

image-push: dist/
	docker buildx build --platform linux/amd64,linux/arm64 \
	-t ghcr.io/kayac/ecspresso:$(IMAGE_VERSION) \
	--push \
	-f Dockerfile .

image-load: dist/
	docker buildx build --platform linux/amd64 \
	-t ghcr.io/kayac/ecspresso:$(IMAGE_VERSION) \
	--load \
	-f Dockerfile .
