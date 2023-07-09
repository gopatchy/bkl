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

todo:
	-git grep -e TODO --and --not -e ignoretodo
