# bkl

bkl (short for Baklava, because it has layers) is a templating configuration language without the templates. It's designed to be simple to read and write with obvious behavior.

Write your configuration in your favorite format: [JSON](https://json.org), [YAML](https://yaml.org/), or [TOML](https://toml.io). Layer configurations on top of each other, even in different formats. Use filenames to define the inheritance. bkl merges your layers together with sane defaults that you can [override](#merge-behavior); have as many layers as you like. Export your results in any supported format for human or machine consumption. Use the CLI directly or in scripts, or automate with the library.

No template tags. No custom syntax. No schemas. No new formats to learn.

## Example

### service.yaml
```yaml
name: myService
addr: 127.0.0.1
port: 8080
```

### service.test.toml
```toml
port = 8081
```

### Run it!
```console
$ bkl service.test.toml
{ "addr": "127.0.0.1", "name": "myService", "port": 8081 }
```

bkl knows that `service.test.toml` inherits from `service.yaml` by the filename pattern, and uses filename extensions to determine format.

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
$ bkl a.b.yaml c.yaml   # a.yaml -> a.b.yaml -> c.yaml -> output
```

### Symlinks

bkl follows symbolic links and evaluates the inherited layers on the other side of the symlink.

```console
$ ln -s a.b.yaml c.yaml
$ bkl c.d.yaml   # a.yaml -> a.b.yaml (c.yaml) -> c.d.yaml -> output

```

### Streams

bkl understands input streams (multi-document YAML files delimited with `---`). To layer them, it has to match up sections between files. It tries the following strategies, in order:
* Custom paths: If you pass `-m`/`--match-key` (example value: `a.b,c.d`), bkl will use its list of paths as match keys.
* K8s paths: If `kind` and `metadata.name` are present, they are used as default match keys.
* Ordering: Stream position is used to match documents.

## Merge Behavior

By default, lists and maps are merged. To change that, use [$patch](https://github.com/edgarsandi/Kubernetes/blob/master/docs/devel/api-conventions.md#strategic-merge-patch) syntax.

### Maps

<table>
  
<tr>

<td>

```yaml
myMap:
  a: 1
```
</td>

<td>

**+**
</td>

<td>

```yaml
myMap:
  b: 2
```
</td>

<td>

**=**
</td>

<td>

```yaml
myMap:
  a: 1
  b: 2
```
</td>

</tr>

<tr></tr>

<tr>

<td>

```yaml
myMap:
  a: 1
```
</td>

<td>

**+**
</td>

<td>

```yaml
myMap:
  b: 2
  $patch: replace
```
</td>

<td>

**=**
</td>

<td>

```yaml
myMap:
  b: 2
```
</td>

</tr>

<tr></tr>

<tr>

<td>

```yaml
myMap:
  a: 1
  b: 2
```
</td>

<td>

**+**
</td>

<td>

```yaml
myMap:
  c: 3
  b: null
```
</td>

<td>

**=**
</td>

<td>

```yaml
myMap:
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
myList:
  - 1
```
</td>

<td>

**+**
</td>

<td>

```yaml
myList:
  - 2
```
</td>

<td>

**=**
</td>

<td>

```yaml
myList:
  - 1
  - 2
```
</td>

</tr>

<tr></tr>

<tr>

<td>

```yaml
myList:
  - 1
```
</td>

<td>

**+**
</td>

<td>

```yaml
myList:
  - 2
  - $patch: replace
```
</td>

<td>

**=**
</td>

<td>

```yaml
myList:
  - 2
```
</td>

</tr>

<tr></tr>

<tr>

<td>

```yaml
myList:
  - x: 1
  - x: 2
```
</td>

<td>

**+**
</td>

<td>

```yaml
myList:
  - x: 3
  - x: 2
    $patch: delete
```
</td>

<td>

**=**
</td>

<td>

```yaml
myList:
  - x: 1
  - x: 3
```
</td>

</tr>

</table>
