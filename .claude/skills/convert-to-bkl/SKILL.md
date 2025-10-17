---
name: Convert to BKL
description: Convert Kubernetes, Helm, Kustomize, or plain YAML configuration files to BKL format with proper layering and inheritance. Use when migrating configs to BKL, converting K8s manifests, or setting up layered configuration structures.
---

# Convert to BKL

This skill guides you through converting configuration files (especially Kubernetes, Helm, and Kustomize) to BKL format with proper layering.

## Overview

The conversion process has several phases:

1. **Find files**: Identify all configuration files to convert
2. **Convert to plain YAML** (if needed): Process Helm/Kustomize to plain YAML
3. **Prep**: Transform patterns to use BKL features
4. **Plan structure**: Determine file layering hierarchy
5. **Create layers**: Use bkl tools to generate layered configs
6. **Validate**: Compare output to original at each step
7. **Polish**: Apply final refinements

## Instructions

### Step 1: Find configuration files

Locate all the configuration files to convert. These might be:
- Plain YAML files in directories like `dev/`, `staging/`, `prod/`
- Helm charts with `Chart.yaml` and `values*.yaml` files
- Kustomize configs with `kustomization.yaml`

### Step 2: Convert to plain YAML (if needed)

**For Helm charts:**
- Create `plain/` directory
- For each environment's values file, run `helm template release CHART_DIR --output-dir plain/ENV -f VALUES_FILE`
- Skip base `values.yaml` if environment-specific files exist
- Result: `plain/ENV/` directories with rendered YAML

**For Kustomize:**
- Run `kubectl kustomize DIR` or `kustomize build DIR`
- Save output to `plain/kustomize-output.yaml`

**For plain YAML:**
- Skip this step

### Step 3: Prep files

Copy files to `prep/` directory and transform them to use BKL patterns. See [prep-patterns.md](prep-patterns.md) for detailed examples.

**Key transformations:**
- Convert lists to maps with `$encode: values`, `$encode: values:KEY`, or `$encode: values:KEY:VALUE`
- Convert argument lists to maps with `$encode: flags`
- Convert embedded JSON/YAML strings to structures with `$encode: json-pretty` or `$encode: yaml`
- Convert base64 strings to readable format with `$encode: base64`
- Use string interpolation `$"text {variable}"` to remove duplication

**Query for documentation:**
```
mcp__bkl-mcp__get type="documentation" id="prep" source="k8s"
```

**Query for common issues:**
```
mcp__bkl-mcp__query keywords="fixit"
```

**Validate each prep file:**
```
mcp__bkl-mcp__compare file1="original/path.yaml" file2="prep/path.yaml"
```

The diff should be empty or contain only ordering differences.

### Step 4: Plan file structure

Determine the layering structure based on inheritance patterns.

**For simple configs** (like namespace files that vary by environment):
```
namespace.yaml (base/prod)
  namespace.staging.yaml
    namespace.staging.dev.yaml
```

**For services with common base:**

First, check if services should share a base layer:
```
mcp__bkl-mcp__intersect selector="kind,metadata.name" files="prep/service1.yaml,prep/service2.yaml"
```

If there's significant overlap, create a shared base:
```
base.yaml (common elements)
  base.service1.yaml (service1 prod)
    base.service1.staging.yaml
      base.service1.staging.dev.yaml
  base.service2.yaml (service2 prod)
    base.service2.staging.yaml
```

**Best practice**: Use production as the base layer. Other environments are expressed as diffs from production.

**Read the planning documentation:**
```
mcp__bkl-mcp__get type="documentation" id="plan" source="k8s"
```

### Step 5: Create BKL layers

Create `bkl/` directory for output.

**For simple files** (no shared base):
```bash
# Base layer (production)
bkl prep/prod/file.yaml --output=bkl/file.yaml

# Derived layers (diffs from parent)
mcp__bkl-mcp__diff selector="kind,metadata.name" baseFile="bkl/file.yaml" targetFile="prep/staging/file.yaml" outputPath="bkl/file.staging.yaml"
mcp__bkl-mcp__diff selector="kind,metadata.name" baseFile="bkl/file.yaml" targetFile="prep/dev/file.yaml" outputPath="bkl/file.staging.dev.yaml"
```

**For files with shared base:**
```bash
# Common base (intersection of all variants)
mcp__bkl-mcp__intersect selector="kind,metadata.name" files="prep/prod/service1.yaml,prep/prod/service2.yaml" outputPath="bkl/base.yaml"

# Service-specific prod layers
mcp__bkl-mcp__diff selector="kind,metadata.name" baseFile="bkl/base.yaml" targetFile="prep/prod/service1.yaml" outputPath="bkl/base.service1.yaml"

# Environment-specific layers
mcp__bkl-mcp__diff selector="kind,metadata.name" baseFile="bkl/base.service1.yaml" targetFile="prep/staging/service1.yaml" outputPath="bkl/base.service1.staging.yaml"
```

**Important**: The `selector` parameter tells intersect/diff how to match documents. Use `kind,metadata.name` for Kubernetes resources.

### Step 6: Validate

After creating each layer, validate it produces the same output as the original:

```
mcp__bkl-mcp__compare file1="original/prod/service.yaml" file2="bkl/base.service.yaml"
mcp__bkl-mcp__compare file1="original/staging/service.yaml" file2="bkl/base.service.staging.yaml"
```

Empty diff means success. Non-empty diff requires investigation and fixes.

**Read validation documentation:**
```
mcp__bkl-mcp__get type="documentation" id="prep-validate" source="k8s"
```

### Step 7: Polish

Apply final refinements to simplify the BKL configs.

**Common polish operations:**
- Merge common overrides using `$match: {}` to apply to all documents
- Use `$matches:` with selectors to apply overrides to specific document types
- Consolidate repeated patterns
- Simplify complex structures

**Read polish documentation:**
```
mcp__bkl-mcp__get type="documentation" id="polish" source="k8s"
```

**Validate again after polish:**
```
mcp__bkl-mcp__compare file1="original/..." file2="bkl/..."
```

## Key Tools

- `mcp__bkl-mcp__query` - Search documentation and tests
- `mcp__bkl-mcp__get` - Get full content of documentation sections
- `mcp__bkl-mcp__intersect` - Find common elements (create base layers)
- `mcp__bkl-mcp__diff` - Find differences (create derived layers)
- `mcp__bkl-mcp__compare` - Validate conversions
- `mcp__bkl-mcp__evaluate` - Evaluate BKL files to YAML/JSON

## Important Notes

- Always validate after each major step
- Use `selector="kind,metadata.name"` for Kubernetes resources
- Production should be the base layer; other environments are diffs
- When evaluation differs from original, don't revert to lists. Find the right `$encode` pattern instead.
- Reference [prep-patterns.md](prep-patterns.md) for detailed pattern examples
