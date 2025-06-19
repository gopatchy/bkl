# Claude Code Memory - BKL Project

## Project Overview
BKL is a layered configuration language parser written in Go that handles YAML/JSON/TOML files with features like:
- File merging and layering with `$parent` directives
- Document processing with string interpolation (`$"..."` syntax)
- Cross-document references and merging
- Environment variable substitution (`$env:VAR`)
- Various output formats (JSON, YAML, TOML)

## Testing Framework
- Tests are in `tests/` directory, each test has its own subdirectory
- Test structure: `a.yaml` (input), `cmd` (command to run), `expected` (expected output)
- Use `./test` to run all tests or `./test <test-name>` for specific test
- For expected failures: use `! bkl` in cmd file and empty expected output
- Test naming patterns: `parent-*`, `interp-*`, `merge-*`, `encode-*`, etc.

## Key Files and Architecture
- `file.go`: File loading and parent resolution
- `document.go`: Document structure and processing
- `parser.go`: Main parser logic and document merging
- `process1.go`/`process2.go`: Document processing phases
- `error.go`: Centralized error definitions with base `Err`

## Error Handling Patterns
- All errors inherit from base `Err` using `fmt.Errorf("message (%w)", Err)`
- Existing error types: `ErrCircularRef`, `ErrMissingFile`, `ErrVariableNotFound`, etc.
- Tests expecting failures use `! bkl` and empty expected output

## Code Style Observations
- Uses standard library `slices` package for modern Go idioms
- Error messages include context (file paths, variable names)
- Consistent naming: functions use camelCase, test directories use kebab-case
- Import organization: standard library first, then third-party

## Commands for Development
- `./test` - Run all tests
- `go build ./cmd/bkl` - Build main binary
- Tests are comprehensive with 160+ test cases covering edge cases

## Interpolation Syntax
- String interpolation: `$"Hello {variable} world"`
- Environment variables: `$env:VARNAME` 
- Cross-document references and path navigation supported
- Missing variables properly return errors