# Quick Tutorial

```bash mdox-exec="bash ./testdata/out.sh"
test output
```

```yaml
abc
sad
```

```
something
sd
```

## Configuration

### Alertmanager

The `--alertmanagers.config` and `--alertmanagers.config-file` flags allow specifying multiple Alertmanagers. Those entries are treated as a single HA group. This means that alert send failure is claimed only if the Ruler fails to send to all instances.

The configuration format is the following:

```yaml mdox-exec="bash ./testdata/out2.sh"
test output2
newline
```

```bash mdox-expect-exit-code=2 mdox-exec="bash ./testdata/out3.sh"
test output3
```

```bash mdox-exec="sed -n '1,3p' ./testdata/out3.sh"
#!/usr/bin/env bash

echo "test output3"
```

```yaml mdox-exec="bash ./testdata/out2.sh --name=queryfrontend.InMemoryResponseCacheConfig"
test output2
newline
```

```bash mdox-exec="cat ./testdata/out3.sh"
#!/usr/bin/env bash

echo "test output3"
exit 2
```
