go := env_var_or_default('GOCMD', 'go')

default: tidy test docs

tidy:
	{{go}} mod tidy
	gofumpt -l -w .

test:
	{{go}} vet ./...
	# golangci-lint run ./...
	{{go}} test -race -coverprofile=cover.out -timeout=60s ./...
	{{go}} tool cover -html=cover.out -o=cover.html

bench:
	{{go}} test -bench=BenchmarkLanguage -benchmem -run=^$ ./...

docs:
	cd docs && {{go}} run .

todo:
	-git grep -e TODO --and --not -e ignoretodo

docker:
	#!/bin/bash -e
	VER=$(git describe --abbrev=0 --tags)
	docker buildx build --target=dist --platform=linux/arm64,linux/amd64 --provenance=false --build-arg=git_tag=$VER --push --tag=ghcr.io/gopatchy/bkl:${VER#v} --tag=ghcr.io/gopatchy/bkl:latest .

pkg:
	#!/bin/bash -e
	VER=$(git describe --abbrev=0 --tags)
	for PLATFORM in linux/arm64 linux/amd64 darwin/arm64 darwin/amd64; do
		echo $PLATFORM
		export GOOS=$(echo $PLATFORM | cut -d / -f 1)
		export GOARCH=$(echo $PLATFORM | cut -d / -f 2)
		DIR=$(mktemp --directory)
		cp LICENSE $DIR
		CGO_ENABLED=0 go build -tags bkl-$VER,bkl-src-pkg -trimpath -ldflags=-extldflags=-static -o $DIR ./...
		cd $DIR
		tar -czf {{justfile_directory()}}/out/bkl-$GOOS-$GOARCH-$VER.tar.gz *
		cd ~-
	done
