# bkl

bkl (short for Baklava, because it has layers) is a templating configuration language without the templates. It's designed to be simple to read and write with obvious behavior.

Write your configuration in your favorite format: [JSON](https://json.org), [YAML](https://yaml.org/), or [TOML](https://toml.io). Layer configurations on top of each other, even in different formats. Use filenames to define the inheritance. bkl merges your layers together with sane defaults that you can [override](#override); have as many layers as you like. Export your results in any supported format for human or machine consumption. Use the CLI directly or in scripts, or automate with the library.

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

bkl knows that service.test.toml inherits from service.yaml by the filename pattern, and uses filename extensions to determine format.

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

## Merge Behavior

By default, lists and maps are merged. To change that, use [$patch](https://github.com/edgarsandi/Kubernetes/blob/master/docs/devel/api-conventions.md#strategic-merge-patch) syntax:

