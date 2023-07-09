go := env_var_or_default('GOCMD', 'go')

default: tidy test

tidy:
	{{go}} mod tidy
	goimports -l -w -gofumpt .

test:
	{{go}} vet ./...
	# Re-enable when support for go 1.21 min/max is ready
	# golangci-lint run ./...
	{{go}} test -race -coverprofile=cover.out -timeout=60s ./...
	{{go}} tool cover -html=cover.out -o=cover.html
	./test

todo:
	-git grep -e TODO --and --not -e ignoretodo
