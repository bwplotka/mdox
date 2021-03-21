Quick Tutorial
==============

```bash mdox-gen-exec="bash ./testdata/out.sh"
a
adf
```

```yaml mdox-gen-lang="go" mdox-gen-type="github.com/bwplotka/mdox/pkg/mdox/testdata.Config"
TO BE DONE
```

```yaml
abc
sad
```

```
something
sd
```

Configuration
-------------

### Alertmanager

The `--alertmanagers.config` and `--alertmanagers.config-file` flags allow specifying multiple Alertmanagers. Those entries are treated as a single HA group. This means that alert send failure is claimed only if the Ruler fails to send to all instances.

The configuration format is the following:

```yaml mdox-gen-exec="bash ./testdata/out2.sh"
alertmanagers:
- http_config:
  api_version: v1
```

```bash expect-exit-code=2 mdox-gen-exec="bash ./testdata/out3.sh"
abc
```