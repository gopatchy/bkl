# bkl

bkl (short for Baklava, because it has layers) is a templating configuration language without the templates. It's designed to be simple to read and write with obvious behavior.

Write your configuration in your favorite format: [JSON](https://json.org), [YAML](https://yaml.org/), or [TOML](https://toml.io). Layer configurations on top of each other, even in different formats. Use filenames to define the inheritance. Have as many layers as you like. bkl merges your layers together with sane default behavior that you can [override](#merge-behavior). Export your results in [any supported format](#output-formats) for human or machine consumption. Use the CLI directly or in scripts, or automate with the [library](https://pkg.go.dev/github.com/gopatchy/bkl#section-documentation).

No template tags. No schemas. No new formats to learn.

## Example

`service.yaml`
```yaml
name: myService
addr: 127.0.0.1
port: 8080
```

`service.test.toml`
```toml
port = 8081
```

### Run it!
```console
$ bkl service.test.toml
{"addr":"127.0.0.1","name":"myService","port":8081}
```

bkl knows that `service.test.toml` inherits from `service.yaml` by the filename pattern, and uses filename extensions to determine formats.

## Output Formats

Output defaults to machine-friendly JSON (you can make that explicit with `-f json`).

### YAML
```console
$ bkl -f yaml service.test.toml
addr: 127.0.0.1
name: myService
port: 8081
```

### TOML
```console
$ bkl -f toml service.test.toml
addr = "127.0.0.1"
name = "myService"
port = 8081
```

### Pretty JSON
```console
$ bkl -f json-pretty service.test.toml
{
  "addr": "127.0.0.1",
  "name": "myService",
  "port": 8081
}
```

## Output Locations

Output goes to stdout by default. Errors always go to stderr.

### File Output
```console
$ bkl -o out.yaml service.test.toml
```

Output format is autodetected from output filename.

## Advanced Inputs

### Multiple Files

Specifying multiple input files evaluates them as normal, then merges them onto each other in order.

```console
$ bkl a.b.yaml c.d.yaml   # (a.yaml + a.b.yaml) + (c.yaml + c.d.yaml)
```

### Symlinks

bkl follows symbolic links and evaluates the inherited layers on the other side of the symlink.

```console
$ ln -s a.b.yaml c.yaml
$ bkl c.d.yaml   # a.yaml + a.b.yaml (c.yaml) + c.d.yaml

```

### Streams

bkl understands input streams (multi-document YAML files delimited with `---`). To layer them, it has to match up sections between files. It tries the following strategies, in order:
* `$match`: specify match fields in the document:
```yaml
$match:
  kind: Service
  metadata:
    name: myService
```
* K8s paths: If `kind` and `metadata.name` are present, they are used as match keys.
* Ordering: Stream position is used to match documents.

## Merge Behavior

By default, lists and maps are merged. To change that, use [$patch](https://github.com/edgarsandi/Kubernetes/blob/master/docs/devel/api-conventions.md#strategic-merge-patch) syntax.

### Maps

<table>
  
<tr>

<td>

```yaml
a: 1
```
</td>

<td>

**+**
</td>

<td>

```yaml
b: 2
```
</td>

<td>

**→**
</td>

<td>

```yaml
a: 1
b: 2
```
</td>

</tr>

<tr></tr>

<tr>

<td>

```yaml
a: 1
```
</td>

<td>

**+**
</td>

<td>

```yaml
b: 2
$patch: replace
```
</td>

<td>

**→**
</td>

<td>

```yaml
b: 2
```
</td>

</tr>

<tr></tr>

<tr>

<td>

```yaml
a: 1
b: 2
```
</td>

<td>

**+**
</td>

<td>

```yaml
c: 3
b: null
```
</td>

<td>

**→**
</td>

<td>

```yaml
a: 1
c: 3
```
</td>

</tr>

</table>

### Lists

<table>
  
<tr>

<td>

```yaml
- 1
```
</td>

<td>

**+**
</td>

<td>

```yaml
- 2
```
</td>

<td>

**→**
</td>

<td>

```yaml
- 1
- 2
```
</td>

</tr>

<tr></tr>

<tr>

<td>

```yaml
- 1
```
</td>

<td>

**+**
</td>

<td>

```yaml
- 2
 $patch: replace
```
</td>

<td>

**→**
</td>

<td>

```yaml
- 2
```
</td>

</tr>

<tr></tr>

<tr>

<td>

```yaml
- x: 1
- x: 2
```
</td>

<td>

**+**
</td>

<td>

```yaml
- x: 3
- x: 2
  $patch: delete
```
</td>

<td>

**→**
</td>

<td>

```yaml
- x: 1
- x: 3
```
</td>

</tr>

</table>

## Advanced Keys & Values

### $required

Use `$required` in lower layers to force upper layers to replace the value.

<table>
  
<tr>

<td>

```yaml
a: 1
b: $required
```
</td>

<td>

**+**
</td>

<td>

```yaml
c: 3
```
</td>

<td>

**→**
</td>

<td>

**Error**

</td>

</tr>

<tr></tr>

<tr>

<td>

```yaml
a: 1
b: $required
```
</td>

<td>

**+**
</td>

<td>

```yaml
b: 2
c: 3
```
</td>

<td>

**→**
</td>

<td>

```yaml
a: 1
b: 2
c: 3
```
</td>

</tr>

</table>

### $merge

Merges in another subtree.

<table>
  
<tr>

<td>

```yaml
foo:
  bar:
    a: 1
zig:
  b: 2
  $merge: foo.bar
```
</td>

<td>

**→**
</td>

<td>

```yaml
foo:
  bar:
    a: 1
zig:
  a: 1
  b: 2
```
</td>

</tr>

</table>

### $replace

Replaces a subtree with one from another location.

<table>
  
<tr>

<td>

```yaml
foo:
  bar:
    a: 1
zig:
  b: 2
  $replace: foo.bar
```
</td>

<td>

**→**
</td>

<td>

```yaml
foo:
  bar:
    a: 1
zig:
  a: 1
```
</td>

</tr>

</table>

### $output

Selects a subtree for output.

<table>
  
<tr>

<td>

```yaml
foo:
  bar:
    $output: true
    a: 1
    b: 2
```
</td>

<td>

**→**
</td>

<td>

```yaml
a: 1
b: 2
```
</td>

</tr>

</table>

Multiple instances of `$output` in a document will generate multiple output documents (delimited with `---`). If the `$output` key has a numeric value, that value is used as the output document index.

Combine `$output` with `$replace` or `$merge` to have hidden "template" subtrees that don't appear in the output but can be copied in as needed. 
