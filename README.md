# [bkl](https://bkl.gopatchy.io/)

bkl (short for Baklava because it has layers) is a templating configuration language without the templates. It's designed to be simple to read and write with obvious behavior.

Write your configuration in your favorite format: [JSON](https://json.org/), [YAML](https://yaml.org/), or [TOML](https://toml.io/). Layer configurations on top of each other, even from different file formats. Use filenames to define the inheritance. Have as many layers as you like. bkl merges your layers together with sane default behavior that you can override. Export your results in any supported format for human or machine consumption. Use the CLI directly or in scripts or automate with the [library](https://pkg.go.dev/github.com/gopatchy/bkl).

[![Go Reference](https://bkl.gopatchy.io/go-reference.svg)](https://pkg.go.dev/github.com/gopatchy/bkl)
[![GitHub: bkl](https://bkl.gopatchy.io/github-bkl.svg)](https://github.com/gopatchy/bkl/)
[![Discord: bkl](https://bkl.gopatchy.io/discord-bkl.svg)](https://discord.gg/UZCFZ37d)

### service.yaml
```yaml
addr: 127.0.0.1
name: myService
port: 8080
```

### service.test.toml
```toml
port = 8081
```

### Run it!
```console
$ bkl service.test.toml
```
```toml
addr = '127.0.0.1'
name = 'myService'
port = 8081
```