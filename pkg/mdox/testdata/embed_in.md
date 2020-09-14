Quick Tutorial
==============

<meta mdox-flag-gen="bash ./pkg/mdox/testdata/out.sh">
```bash
a
```
<meta mdox-cfg-go-gen="github.com/bwplotka/mdox/pkg/mdox/testdata.Config">

Configuration
-------------

### Alertmanager

The `--alertmanagers.config` and `--alertmanagers.config-file` flags allow specifying multiple Alertmanagers. Those entries are treated as a single HA group. This means that alert send failure is claimed only if the Ruler fails to send to all instances.

The configuration format is the following:

<meta mdox-flag-gen="bash ./pkg/mdox/testdata/out2.sh">
```yaml
alertmanagers:
- http_config:
  api_version: v1
```
