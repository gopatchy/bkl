# BKL Public API Analysis and Suggestions

## Current Public API

### Core Types

1. **BKL** - Main entry point for the library
   - `New() (*BKL, error)` - Constructor
   - `Documents() []*Document` - Get parsed documents

### Package-level Variables

- `Debug bool` - Controls debug logging for all BKL operations (initialized from BKL_DEBUG env var)
   
2. **Document** - Represents a parsed configuration document
   - `Process([]*Document, map[string]string) ([]*Document, error)` - Process with merge docs and env vars
   - `String() string` - String representation

3. **Format** - Configuration format handling
   - `GetFormat(string) (*Format, error)` - Get format by name

### Main Operations

1. **File Operations**
   - `MergeFileLayers(fs.FS, string) error` - Merge layers from a file
   - `OutputToFile(string, string, map[string]string) error` - Write output to file
   - `Evaluate(fs.FS, []string, bool, string, string, string, map[string]string) ([]byte, error)` - Full evaluation pipeline
   - `EvaluateToData(...)` - Same as Evaluate but returns data instead of bytes

2. **Specialized Operations**
   - `Diff(any, any, map[string]string) (any, error)` - Compare two configurations
   - `DiffFiles(fs.FS, string, string) (any, error)` - Compare two files
   - `Intersect(any, any) (any, error)` - Find common elements
   - `IntersectFiles(fs.FS, []string) (any, error)` - Intersect multiple files
   - `Required(any) (any, error)` - Extract required fields
   - `RequiredFile(fs.FS, string) (any, error)` - Extract required from file

3. **Utility Methods**
   - `PreparePathsFromCwd([]string, string) ([]string, error)` - Path preparation
   - `GetOSEnv() map[string]string` - Get environment variables
   - `Ext(string) string` - Get file extension
   - `FileMatch(fs.FS, string) (string, string, error)` - Match file patterns

## Issues with Current API

1. **Inconsistent Naming**
   - Some methods use `File` suffix (DiffFiles, RequiredFile) while others don't (MergeFileLayers)
   - Mix of verb-first (GetFormat) and noun-first (OutputToFile) naming

2. **Complex Method Signatures**
   - `Evaluate` has 7 parameters - difficult to use correctly
   - Many methods take `fs.FS` as first parameter but it's not consistent

3. **Missing Convenience Methods**
   - No simple way to evaluate a single file
   - Unclear that BKL is stateful and supports incremental merging
   - No validation-only mode

4. **Unclear Stateful Design**
   - Not obvious that BKL accumulates documents via MergeFileLayers
   - No way to reset BKL state
   - mergeDocument is private, limiting streaming use cases

5. **Unclear Separation of Concerns**
   - BKL mixes parsing, evaluation, and output concerns
   - Document processing is split between BKL and Document types

## Suggested Improvements

### 1. Simplified Primary API

```go
// Simple evaluation methods
func (b *BKL) EvaluateFile(path string) ([]byte, error)
func (b *BKL) EvaluateFiles(paths ...string) ([]byte, error)
func (b *BKL) EvaluateString(content, format string) ([]byte, error)
func (b *BKL) EvaluateReader(r io.Reader, format string) ([]byte, error)

// Keep complex Evaluate for backward compatibility
func (b *BKL) Evaluate(...) ([]byte, error)
```

### 2. Consistent File Operations

```go
// Rename for consistency
func (b *BKL) MergeFile(fsys fs.FS, path string) error  // was MergeFileLayers
func (b *BKL) DiffFiles(fsys fs.FS, path1, path2 string) (any, error)  // keep as is
func (b *BKL) IntersectFiles(fsys fs.FS, paths ...string) (any, error)  // variadic
func (b *BKL) RequiredFromFile(fsys fs.FS, path string) (any, error)  // was RequiredFile
```

### 3. Validation Support

```go
type ValidationError struct {
    Path    string
    Message string
}

func (b *BKL) Validate(data any) []ValidationError
func (b *BKL) ValidateFile(fsys fs.FS, path string) []ValidationError
```

### 4. Clarify Stateful/Streaming Nature

The BKL is already stateful and supports incremental document merging, but this isn't clear from the API. Suggestions:

```go
// Make the stateful nature explicit with better naming
func (b *BKL) AddFile(fsys fs.FS, path string) error  // was MergeFileLayers
func (b *BKL) AddDocument(doc *Document) error  // was mergeDocument (private)
func (b *BKL) Clear()  // Reset state
func (b *BKL) DocumentCount() int  // Get number of accumulated documents

// Consider making mergeDocument public to support streaming use cases
func (b *BKL) MergeDocument(doc *Document) error
```

This would make it clear that:
- BKL accumulates state as files are added
- Multiple files can be merged incrementally
- The parser can be reused by clearing state

### 5. Error Improvements

```go
// Make errors more structured
type BKLError struct {
    Type    string  // "parse", "merge", "eval", etc.
    Path    string  // file path if applicable
    Line    int     // line number if applicable
    Column  int     // column if applicable
    Message string
    Cause   error
}

func (e *BKLError) Error() string
func (e *BKLError) Unwrap() error
```

### 6. Format Registration

```go
// Allow custom format registration
func RegisterFormat(name string, format *Format) error
func UnregisterFormat(name string) error
func ListFormats() []string
```

### 7. Helper Functions

```go
// Package-level convenience functions
func EvaluateFile(path string) ([]byte, error)
func EvaluateFiles(paths ...string) ([]byte, error)
func DiffFiles(path1, path2 string) (any, error)
```

### 8. Document Methods

```go
// Add more useful methods to Document
func (d *Document) GetString(path string) (string, error)
func (d *Document) GetInt(path string) (int, error)
func (d *Document) GetBool(path string) (bool, error)
func (d *Document) GetMap(path string) (map[string]any, error)
func (d *Document) GetList(path string) ([]any, error)
func (d *Document) Set(path string, value any) error
func (d *Document) Delete(path string) error
func (d *Document) Exists(path string) bool
```

## Implementation Priority

1. **High Priority** (Breaking changes, do first)
   - Add simplified evaluation methods
   - Consistent file operation naming

2. **Medium Priority** (Additions, backward compatible)
   - Clarify stateful nature with better method names
   - Validation support
   - Document accessor methods
   - Package-level helpers
   - Structured errors

3. **Low Priority** (Nice to have)
   - Format registration

## Migration Path

1. Keep existing methods but mark complex ones as deprecated
2. Provide migration guide showing old vs new patterns
3. Use semantic versioning - this would be a v2.0.0 release
4. Consider providing a compatibility package for easier migration

## Example Usage After Changes

```go
// Simple case
result, err := bkl.EvaluateFile("config.yaml")

// BKL with simple methods
b, _ := bkl.New()
result, err := b.EvaluateFiles("base.yaml", "prod.yaml")

// Enable debug logging
bkl.Debug = true
result, err := b.EvaluateFile("config.yaml")

// Stateful/incremental usage
b, _ := bkl.New()
b.AddFile(os.DirFS("."), "base.yaml")
b.AddFile(os.DirFS("."), "overrides.yaml")
result, err := b.OutputToFile("output.json", "json", nil)

// Validation
errors := b.ValidateFile(os.DirFS("."), "config.yaml")
for _, err := range errors {
    log.Printf("Validation error at %s: %s", err.Path, err.Message)
}

// Document manipulation
docs := parser.Documents()
value, _ := docs[0].GetString("server.host")
docs[0].Set("server.port", 8080)
```

This would make the API more intuitive, consistent, and easier to use while maintaining the power of the current implementation.