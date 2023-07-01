# bkl

bkl (short for Baklava, because it has layers) is a templating configuration language without the templates. It's designed to be simple to read and write with obvious behavior.

Write your configuration in your favorite format: [JSON](https://json.org), [YAML](https://yaml.org/), or [TOML](https://toml.io). Layer configurations on top of each other, even in different formats. Use filenames to define the inheritance. bkl merges your layers together with sane defaults that you can [override](#override). Export your results in any supported format for human or machine consumption. Use the CLI directly or in scripts, or automate with the library.

No template tags. No custom syntax. No schemas. No new formats to learn.

## Example
#### **`service.yaml`**
```yaml
name: myService
addr: 127.0.0.1
port: 8080
```

#### **`service.test.toml`**
```toml
port = 8081
```

Then run `bkl` to evaluate:
```console
$ bkl service.test.toml
{ "name": "myService", "addr": "127.0.0.1", "port": 8081 }
```
