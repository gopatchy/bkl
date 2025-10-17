# Prep Patterns Reference

This document contains detailed examples of converting configuration patterns to BKL format during the prep phase.

## Overview

The prep phase transforms original configs to use BKL features that make layering easier. The key principle: **convert lists to maps whenever possible** so that upper layers can selectively override individual items.

## Port Lists

### Simple ports (no name field)

**From:**
```yaml
ports:
  - port: 80
    protocol: TCP
  - port: 443
    protocol: TCP
```

**To:**
```yaml
ports:
  http:
    port: 80
    protocol: TCP
  https:
    port: 443
    protocol: TCP
  $encode: values
```

### Ports with name field

**From:**
```yaml
ports:
  - name: http
    port: 80
    protocol: TCP
  - name: https
    port: 443
    protocol: TCP
```

**To:**
```yaml
ports:
  http:
    port: 80
    protocol: TCP
  https:
    port: 443
    protocol: TCP
  $encode: values:name
```

The `name` field is removed from each port and used as the map key instead.

### Container ports (unnamed)

**From:**
```yaml
ports:
  - containerPort: 8080
  - containerPort: 9090
```

**To:**
```yaml
ports:
  api: 8080
  metrics: 9090
  $encode: values::containerPort
```

Note: `values::containerPort` (double colon) means the map key becomes just the value, and `containerPort` is the field to populate.

### Nested name fields

**From:**
```yaml
metrics:
  - resource:
      name: cpu
      target:
        averageUtilization: 70
        type: Utilization
    type: Resource
  - resource:
      name: memory
      target:
        averageUtilization: 80
        type: Utilization
    type: Resource
```

**To:**
```yaml
metrics:
  cpu:
    resource:
      target:
        averageUtilization: 70
        type: Utilization
    type: Resource
  memory:
    resource:
      target:
        averageUtilization: 80
        type: Utilization
    type: Resource
  $encode: values:resource.name
```

Use dotted path notation (`resource.name`) for nested fields.

## Environment Variables

### Simple name/value pairs

**From:**
```yaml
env:
  - name: DB_HOST
    value: postgres
  - name: DB_PORT
    value: "5432"
```

**To:**
```yaml
env:
  DB_HOST: postgres
  DB_PORT: "5432"
  $encode: values:name:value
```

The syntax `values:name:value` means: map keys become the `name` field, map values become the `value` field.

### Environment variables with valueFrom

**From:**
```yaml
env:
  - name: ANALYTICS_KEY
    valueFrom:
      secretKeyRef:
        key: analytics-key
        name: web-secrets
  - name: API_URL
    value: https://api.example.com
```

**To:**
```yaml
env:
  ANALYTICS_KEY:
    valueFrom:
      secretKeyRef:
        key: analytics-key
        name: web-secrets
  API_URL:
    value: https://api.example.com
  $encode: values:name
```

When some entries have `value` and others have `valueFrom`, use `values:name` instead of `values:name:value`.

## Arguments and Flags

### Command line flags

**From:**
```yaml
args:
  - --config=/app/config/app.properties
  - --feature=auth
  - --feature=metrics
  - --feature=rate-limiting
  - --log-format=json
  - --log-level=2
  - --verbose
```

**To:**
```yaml
args:
  config: /app/config/app.properties
  feature:
    - auth
    - metrics
    - rate-limiting
  log-format: json
  log-level: 2
  verbose: ""
  $encode: flags
```

The `$encode: flags` directive:
- Converts map entries to `--key=value` format
- Supports repeated flags (lists become multiple `--key=value` entries)
- Empty string values become bare flags (just `--verbose`)
- Values are automatically converted to strings

## Path Lists

Use meaningful keys even if they require quoting:

**From:**
```yaml
paths:
  - backend:
      service:
        name: api-service
    path: /api(/|$)(.*)
    pathType: Prefix
  - backend:
      service:
        name: auth-service
    path: /auth(/|$)(.*)
    pathType: Prefix
  - backend:
      service:
        name: frontend-service
    path: /static
    pathType: Prefix
```

**To:**
```yaml
paths:
  /static:
    backend:
      service:
        name: frontend-service
    pathType: Prefix
  /api(/|$)(.*):
    backend:
      service:
        name: api-service
    pathType: Prefix
  /auth(/|$)(.*):
    backend:
      service:
        name: auth-service
    pathType: Prefix
  $encode: values:path
```

## Embedded Formats

### JSON strings to structures

**From:**
```yaml
app.config.json: |
  {
    "apiUrl": "http://api-service",
    "debugMode": true,
    "environment": "development",
    "features": {
      "analytics": false,
      "caching": false
    }
  }
```

**To:**
```yaml
app.config.json:
  apiUrl: http://api-service
  debugMode: true
  environment: development
  features:
    analytics: false
    caching: false
  $encode: json-pretty
```

Use `$encode: json-pretty` for formatted JSON or `$encode: json` for compact.

### YAML strings to structures

**From:**
```yaml
prometheus.yml: |
  global:
    evaluation_interval: 15s
    scrape_interval: 15s
```

**To:**
```yaml
prometheus.yml:
  global:
    evaluation_interval: 15s
    scrape_interval: 15s
  $encode: yaml
```

### Properties files

**From:**
```yaml
app.properties: |
  cache.ttl=3600
  environment=production
  logging.format=json
  logging.level=warn
  logging.output=stdout
```

**To:**
```yaml
app.properties:
  cache.ttl: 3600
  environment: production
  logging.format: json
  logging.level: warn
  logging.output: stdout
  $encode: properties
```

### Base64 encoding

**From:**
```yaml
database-url: cG9zdGdyZXNxbDovL3Byb2R1c2VyOnByb2RwYXNzQHByb2QtZGItY2x1c3Rlci5yZHMuYW1hem9uYXdzLmNvbTo1NDMyL3Byb2RkYg==
```

**To:**
```yaml
database-url:
  $value: postgresql://produser:prodpass@prod-db-cluster.rds.amazonaws.com:5432/proddb
  $encode: base64
```

## String Interpolation

Remove duplication by using `$"..."` interpolation:

**From:**
```yaml
labels:
  environment: prod
namespace: api-prod
```

**To:**
```yaml
labels:
  environment: prod
namespace: $"api-{labels.environment}"
```

Reference other fields using dotted paths inside `{...}` within `$"..."` strings.

## Common Mistakes (from fixit.yaml)

### ❌ Wrong: Unnamed ports with wrong encode

**Original:**
```yaml
ports:
  - containerPort: 8080
```

**Bad conversion:**
```yaml
ports:
  api:
    containerPort: 8080
  $encode: values:containerPort
```

**✅ Correct:**
```yaml
ports:
  api: 8080
  $encode: values::containerPort
```

Use double colon (`::`) when you want the key to become just a value without nesting.

### ❌ Wrong: Environment variables without :value

**Original:**
```yaml
env:
  - name: DB_HOST
    value: postgres
  - name: DB_PORT
    value: "5432"
```

**Bad conversion:**
```yaml
env:
  DB_HOST: postgres
  DB_PORT: "5432"
  $encode: values:name
```

**✅ Correct:**
```yaml
env:
  DB_HOST: postgres
  DB_PORT: "5432"
  $encode: values:name:value
```

When both name and value are simple fields, use `values:name:value`.

### ❌ Wrong: $encode at wrong nesting level

**Original:**
```yaml
imagePullSecrets:
  - name: regcred
```

**Bad conversion:**
```yaml
imagePullSecrets:
  regcred:
    $encode: values:name
```

**✅ Correct:**
```yaml
imagePullSecrets:
  regcred: {}
  $encode: values:name
```

The `$encode` directive applies to the map containing the entries, not to individual entries.

## Tips

1. **Always validate** after converting: use `mcp__bkl-mcp__compare` to check the prep file produces the same output as the original

2. **Choose meaningful keys**: When converting lists to maps, pick keys that make semantic sense (like `http`/`https` for ports, environment variable names for env vars)

3. **Use the right encode variant**:
   - `$encode: values` - List of objects, key is arbitrary
   - `$encode: values:KEY` - List where KEY field becomes the map key
   - `$encode: values:KEY:VALUE` - List where KEY field becomes key, VALUE field becomes value
   - `$encode: values::FIELD` - List where map key becomes the value and FIELD is the field name
   - `$encode: flags` - Command-line style flags
   - `$encode: json-pretty` / `$encode: yaml` / `$encode: properties` - Embedded formats

4. **Query for help**: Use `mcp__bkl-mcp__query keywords="fixit"` to find solutions to common conversion problems
