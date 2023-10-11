## Inheritance

```
┌─────────────────┐ ┌─────────────────┐ ┌─────────────────┐
│ deployment.yaml │ │  service.yaml   │ │  ingress.yaml   │
└─────────────────┘ └─────────────────┘ └─────────────────┘
         │                   │                   │
         └───────────────────┼───────────────────┘
                             ▼
                    ┌─────────────────┐
                    │ resources.yaml  │ x2
                    └─────────────────┘
                             │
                 ┌───────────┴────────────┐
                 ▼                        ▼
     ┌──────────────────────┐ ┌──────────────────────┐
     │ resources.user1.toml │ │ resources.user2.toml │
     └──────────────────────┘ └──────────────────────┘
                 │                        │
                 └───────────┬────────────┘
                             ▼
                    ┌─────────────────┐
                    │    all.yaml     │
                    └─────────────────┘
                             │
                             ▽
                    ┌─────────────────┐
                    │  variant.yaml   │
                    └─────────────────┘
                             ║
                             ▽
                    ╔═════════════════╗
                    ║      json       ║
                    ╚═════════════════╝
```

## Command

```console
$ bkl all.yaml variant.yaml
```

## Order of Operations

- Load `all.yaml`
  - Load `resources.user1.toml`
    - Load `resources.yaml` (1)
      - Load `deployment.yaml` (1)
      - Load `service.yaml` (1)
      - Load `ingress.yaml` (1)
  - Load `resources.user2.toml`
    - Load `resources.yaml` (2)
      - Load `deployment.yaml` (2)
      - Load `service.yaml` (2)
      - Load `ingress.yaml` (2)
- Load `variant.yaml`
