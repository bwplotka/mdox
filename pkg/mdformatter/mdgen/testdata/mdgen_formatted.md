# Quick Tutorial

```bash mdox-gen-exec="bash ./testdata/out.sh"
test output
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

## Configuration

### Alertmanager

The `--alertmanagers.config` and `--alertmanagers.config-file` flags allow specifying multiple Alertmanagers. Those entries are treated as a single HA group. This means that alert send failure is claimed only if the Ruler fails to send to all instances.

The configuration format is the following:

```yaml mdox-gen-exec="bash ./testdata/out2.sh"
test output2
newline
```

```bash mdox-expect-exit-code=2 mdox-gen-exec="bash ./testdata/out3.sh"
test output3
```

```bash mdox-gen-exec="sed -n '1,3p' ./testdata/out3.sh"
#!/usr/bin/env bash

echo "test output3"
```
