Quick Tutorial
==============

```bash mdox-exec="bash ./testdata/out.sh"
a
adf
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

```yaml mdox-exec="bash ./testdata/out2.sh"
alertmanagers:
- http_config:
  api_version: v1
```

```bash mdox-expect-exit-code=2 mdox-exec="bash ./testdata/out3.sh"
abc
```

```bash mdox-exec="sed -n '1,3p' ./testdata/out3.sh"
```

```yaml mdox-exec="bash ./testdata/out2.sh --name=queryfrontend.InMemoryResponseCacheConfig"
```

```bash mdox-exec="cat ./testdata/out3.sh"
```
