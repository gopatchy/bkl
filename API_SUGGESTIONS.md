# BKL Public API Analysis and Suggestions

## Current Public API

### Package-level Functions

- `New() (*BKL, error)` - Constructor for BKL instance
- `GetDocSections() ([]DocSection, error)` - Get embedded documentation sections
- `GetTests() (map[string]*TestCase, error)` - Get embedded test cases

### Package-level Variables

- `Debug bool` - Controls debug logging for all BKL operations (initialized from BKL_DEBUG env var)

### Exported Types

1. **BKL** - The main struct that reads input documents, merges layers, and generates outputs
2. **Document** - Represents a document with ID, Parents, and Data fields
   - `String() string` - String representation (implements fmt.Stringer)
3. **TestCase** - Test case structure
4. **DocSection** - Documentation section structure
5. **ContentItem** - Content item within a documentation section
6. **Example** - Example within a content item
7. **GridRow** - Grid row for examples
8. **GridItem** - Individual grid item

### Exported Error Variables

- `Err` - Base error; every error in bkl inherits from this
- `ErrCircularRef` - Circular reference error
- `ErrConflictingParent` - Conflicting $parent error
- `ErrExtraEntries` - Extra entries error
- `ErrExtraKeys` - Extra keys error
- `ErrInvalidArguments` - Invalid arguments error
- `ErrInvalidDirective` - Invalid directive error
- `ErrInvalidIndex` - Invalid index error
- `ErrInvalidFilename` - Invalid filename error
- `ErrInvalidInput` - Invalid input error
- `ErrInvalidType` - Invalid type error
- `ErrInvalidParent` - Invalid $parent error
- `ErrInvalidRepeat` - Invalid $repeat error
- `ErrMarshal` - Encoding error
- `ErrRefNotFound` - Reference not found error
- `ErrMissingEnv` - Missing environment variable error
- `ErrMissingFile` - Missing file error
- `ErrMissingMatch` - Missing $match error
- `ErrMultiMatch` - Multiple documents $match error
- `ErrNoMatchFound` - No document/entry matched $match error
- `ErrNoCloneFound` - No document/entry matched $clone error
- `ErrOutputFile` - Error opening output file
- `ErrRequiredField` - Required field not set error
- `ErrUnknownFormat` - Unknown format error
- `ErrUnmarshal` - Decoding error
- `ErrUselessOverride` - Useless override error
- `ErrVariableNotFound` - Variable not found error

### BKL Methods

1. **File Operations**
   - `MergeFileLayers(fs.FS, string) error` - Merge layers from a file
   - `OutputToFile(string, string, map[string]string) error` - Write output to file
   - `Evaluate(fs.FS, []string, bool, string, string, string, map[string]string) ([]byte, error)` - Full evaluation pipeline
   - `EvaluateToData(fs.FS, []string, bool, string, string, string, map[string]string) (any, error)` - Same as Evaluate but returns data instead of bytes
   - `FormatOutput(data any, format string) ([]byte, error)` - Format data to specified output format
   - `FileMatch(fs.FS, string) (string, string, error)` - Match file patterns

2. **Specialized Operations**
   - `DiffFiles(fs.FS, string, string) (any, error)` - Compare two files
   - `IntersectFiles(fs.FS, []string) (any, error)` - Intersect multiple files
   - `RequiredFile(fs.FS, string) (any, error)` - Extract required from file

3. **Utility Methods**
   - `PreparePathsFromCwd([]string, string) ([]string, error)` - Path preparation
   - `GetOSEnv() map[string]string` - Get environment variables
   - `Ext(string) string` - Get file extension


## Issues with Current API

1. **Inconsistent Naming**
   - Some methods use `File` suffix (DiffFiles, RequiredFile) while others don't (MergeFileLayers)
   - Mix of verb-first and noun-first naming

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

### 6. Format Support

```go
// List supported formats
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
doc := &Document{Data: config}
value, _ := doc.GetString("server.host")
doc.Set("server.port", 8080)
```

This would make the API more intuitive, consistent, and easier to use while maintaining the power of the current implementation.