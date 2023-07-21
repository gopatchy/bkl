go := env_var_or_default('GOCMD', 'go')

default: tidy test

tidy:
	{{go}} mod tidy
	goimports -l -w -gofumpt .

test:
	{{go}} vet ./...
	golangci-lint run ./...
	{{go}} test -race -coverprofile=cover.out -timeout=60s ./...
	{{go}} tool cover -html=cover.out -o=cover.html
	./test

polytest:
	@just go=go1.21rc2
	@just go=go1.20.5
	@just go=go1.19.10
	@just go=go1.18.10

fuzz:
	{{go}} test -fuzz FuzzParser

fuzz-save:
	cp ~/.cache/go-build/fuzz/github.com/gopatchy/bkl/FuzzParser/* testdata/fuzz/FuzzParser/
	rm ~/.cache/go-build/fuzz/github.com/gopatchy/bkl/FuzzParser/*

todo:
	-git grep -e TODO --and --not -e ignoretodo

docker:
	#!/bin/bash -ex
	VER=$(git tag --sort=v:refname | tail -1)
	docker buildx build --target=dist --platform=linux/arm64,linux/amd64 --provenance=false --build-arg=git_tag=$VER --push --tag=ghcr.io/gopatchy/bkl:${VER#v} --tag=ghcr.io/gopatchy/bkl:latest pkg
