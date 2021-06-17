---
weight: 1
type: docs
title: Quick Tutorial
slug: /quick-tutorial.md
menu: thanos
excerpt: 'Thanos:'
---

# Quick Tutorial

Feel free to check the free, in-browser interactive tutorial [as Katacoda Thanos Course]($$-https://katacoda.com/bwplotka/courses/thanos-testdata/not_formatted.md-$$) We will be progressively updating our Katacoda Course with more scenarios.

On top of this feel free to go through our tutorial presented here:

Yolo link $$-https://thanos.io/some-testdata/not_formatted.md-$$ Yolo email something@gmail.com

* [ ] task
  * [X] task

### Prometheus

Thanos is based on Prometheus. With Thanos you use more or less Prometheus features depending on the deployment model, however Prometheus always stays as integral foundation for *collecting metrics* and alerting using local data.

Thanos bases itself on vanilla [Prometheus]($$-https://prometheus.io/-testdata/not_formatted.md-$$) (v2.2.1+). We plan to support *all* Prometheus version beyond this version.

NOTE: It is highly recommended to use Prometheus v2.13+ due to Prometheus remote read improvements.

Always make sure to run Prometheus as recommended by Prometheus team, so:

* Put Prometheus in the same failure domain. This means same network, same datacenter as monitoring services.
* Use persistent disk to persist data across Prometheus restarts.
* Use local compaction for longer retentions.
* Do not change min TSDB block durations.
* Do not scale out Prometheus unless necessary. Single Prometheus is highly efficient (:

We recommend using Thanos when you need to scale out your Prometheus instance.

### Components

Following the [KISS]($$-https://en.wikipedia.org/wiki/KISS_principle-testdata/not_formatted.md-$$) and Unix philosophies, Thanos is made of a set of components with each filling a specific role.

* Sidecar: connects to Prometheus, reads its data for query and/or uploads it to cloud storage.
* Store Gateway: serves metrics inside of a cloud storage bucket.
* Compactor: compacts, downsamples and applies retention on the data stored in cloud storage bucket.
* Receiver: receives data from Prometheus's remote-write WAL, exposes it and/or upload it to cloud storage.
* Ruler/Rule: evaluates recording and alerting rules against data in Thanos for exposition and/or upload.
* Querier/Query: implements Prometheus's v1 API to aggregate data from the underlying components.

See those components on this diagram:

<img src="$$-img/arch.jpg-testdata/not_formatted.md-$$" class="img-fluid" alt="architecture overview"/>

![img]($$-img/arch.jpg-testdata/not_formatted.md-$$)

### [Sidecar]($$-components/sidecar.md-testdata/not_formatted.md-$$)

Thanos integrates with existing Prometheus servers through a [Sidecar process]($$-https://docs.microsoft.com/en-us/azure/architecture/patterns/sidecar#solution-testdata/not_formatted.md-$$), which runs on the same machine or in the same pod as the Prometheus server.

The purpose of the Sidecar is to backup Prometheus data into an Object Storage bucket, and give other Thanos components access to the Prometheus metrics via a gRPC API.

The Sidecar makes use of the `reload` Prometheus endpoint. Make sure it's enabled with the flag `--web.enable-lifecycle`.

#### External storage

The following configures the sidecar to write Prometheus's data into a configured object storage:

```bash
thanos sidecar \
    --tsdb.path            /var/prometheus \          # TSDB data directory of Prometheus
    --prometheus.url       "http://localhost:9090" \  # Be sure that the sidecar can use this url!
    --objstore.config-file bucket_config.yaml \       # Storage configuration for uploading data
```

The format of YAML file depends on the provider you choose. Examples of config and up-to-date list of storage types Thanos supports is available [here]($$-storage.md-testdata/not_formatted.md-$$).

Rolling this out has little to zero impact on the running Prometheus instance. It is a good start to ensure you are backing up your data while figuring out the other pieces of Thanos.

If you are not interested in backing up any data, the `--objstore.config-file` flag can simply be omitted.

* *[Example Kubernetes manifests using Prometheus operator]($$-https://github.com/coreos/prometheus-operator/tree/master/example/thanos-testdata/not_formatted.md-$$)*
* *[Example Deploying sidecar using official Prometheus Helm Chart]($$-/tutorials/kubernetes-helm/README.md-testdata/not_formatted.md-$$)*
* *[Details & Config for other object stores]($$-storage.md-testdata/not_formatted.md-$$)*

#### Store API

The Sidecar component implements and exposes a gRPC *[Store API]($$-/pkg/store/storepb/rpc.proto#L19-testdata/not_formatted.md-$$)*. The sidecar implementation allows you to query the metric data stored in Prometheus.

Let's extend the Sidecar in the previous section to connect to a Prometheus server, and expose the Store API.

```bash
thanos sidecar \
    --tsdb.path                 /var/prometheus \
    --objstore.config-file      bucket_config.yaml \       # Bucket config file to send data to
    --prometheus.url            http://localhost:9090 \    # Location of the Prometheus HTTP server
    --http-address              0.0.0.0:19191 \            # HTTP endpoint for collecting metrics on the Sidecar
    --grpc-address              0.0.0.0:19090              # GRPC endpoint for StoreAPI
```

* *[Example Kubernetes manifests using Prometheus operator]($$-https://github.com/coreos/prometheus-operator/tree/master/example/thanos-testdata/not_formatted.md-$$)*

### Uploading old metrics.

When sidecar is run with the `--shipper.upload-compacted` flag it will sync all older existing blocks from the Prometheus local storage on startup. NOTE: This assumes you never run sidecar with block uploading against this bucket. Otherwise manual steps are needed to remove overlapping blocks from the bucket. Those will be suggested by the sidecar verification process.

#### External Labels

Prometheus allows the configuration of "external labels" of a given Prometheus instance. These are meant to globally identify the role of that instance. As Thanos aims to aggregate data across all instances, providing a consistent set of external labels becomes crucial!

Every Prometheus instance must have a globally unique set of identifying labels. For example, in Prometheus's configuration file:

```yaml
global:
  external_labels:
    region: eu-west
    monitor: infrastructure
    replica: A
```

### [Querier/Query]($$-components/query.md-testdata/not_formatted.md-$$)

Now that we have setup the Sidecar for one or more Prometheus instances, we want to use Thanos' global [Query Layer]($$-components/query.md-testdata/not_formatted.md-$$) to evaluate PromQL queries against all instances at once.

The Query component is stateless and horizontally scalable and can be deployed with any number of replicas. Once connected to the Sidecars, it automatically detects which Prometheus servers need to be contacted for a given PromQL query.

Query also implements Prometheus's official HTTP API and can thus be used with external tools such as Grafana. It also serves a derivative of Prometheus's UI for ad-hoc querying and stores status.

Below, we will set up a Query to connect to our Sidecars, and expose its HTTP UI.

```bash
thanos query \
    --http-address 0.0.0.0:19192 \                                # HTTP Endpoint for Query UI
    --store        1.2.3.4:19090 \                                # Static gRPC Store API Address for the query node to query
    --store        1.2.3.5:19090 \                                # Also repeatable
    --store        dnssrv+_grpc._tcp.thanos-store.monitoring.svc  # Supports DNS A & SRV records
```

Go to the configured HTTP address that should now show a UI similar to that of Prometheus. If the cluster formed correctly you can now query across all Prometheus instances within the cluster. You can also check the Stores page to check up on your stores.

#### Deduplicating Data from Prometheus HA pairs

The Query component is also capable of deduplicating data collected from Prometheus HA pairs. This requires configuring Prometheus's `global.external_labels` configuration block to identify the role of a given Prometheus instance.

A typical choice is simply the label name "replica" while letting the value be whatever you wish. For example, you might set up the following in Prometheus's configuration file:

```yaml
global:
  external_labels:
    region: eu-west
    monitor: infrastructure
    replica: A
# ...
```

In a Kubernetes stateful deployment, the replica label can also be the pod name.

Reload your Prometheus instances, and then, in Query, we will define `replica` as the label we want to enable deduplication to occur on:

```bash
thanos query \
    --http-address        0.0.0.0:19192 \
    --store               1.2.3.4:19090 \
    --store               1.2.3.5:19090 \
    --query.replica-label replica  # Replica label for de-duplication
    --query.replica-label replicaX # Supports multiple replica labels for de-duplication
```

Go to the configured HTTP address, and you should now be able to query across all Prometheus instances and receive de-duplicated data.

* *[Example Kubernetes manifest]($$-https://github.com/thanos-io/kube-thanos/blob/master/manifests/thanos-query-deployment.yaml-testdata/not_formatted.md-$$)*

#### Communication Between Components

The only required communication between nodes is for Thanos Querier to be able to reach gRPC storeAPIs you provide. Thanos Querier periodically calls Info endpoint to collect up-to-date metadata as well as checking the health of given StoreAPI. The metadata includes the information about time windows and external labels for each node.

There are various ways to tell query component about the StoreAPIs it should query data from. The simplest way is to use a static list of well known addresses to query. These are repeatable so can add as many endpoint as needed. You can put DNS domain prefixed by `dns+` or `dnssrv+` to have Thanos Query do an `A` or `SRV` lookup to get all required IPs to communicate with.

```bash
thanos query \
    --http-address 0.0.0.0:19192 \              # Endpoint for Query UI
    --grpc-address 0.0.0.0:19092 \              # gRPC endpoint for Store API
    --store        1.2.3.4:19090 \              # Static gRPC Store API Address for the query node to query
    --store        1.2.3.5:19090 \              # Also repeatable
    --store        dns+rest.thanos.peers:19092  # Use DNS lookup for getting all registered IPs as separate StoreAPIs
```

Read more details [here]($$-service-discovery.md-testdata/not_formatted.md-$$).

* *[Example Kubernetes manifests using Prometheus operator]($$-https://github.com/coreos/prometheus-operator/tree/master/example/thanos-testdata/not_formatted.md-$$)*

### [Store Gateway]($$-components/store.md-testdata/not_formatted.md-$$)

As the sidecar backs up data into the object storage of your choice, you can decrease Prometheus retention and store less locally. However we need a way to query all that historical data again. The store gateway does just that by implementing the same gRPC data API as the sidecars but backing it with data it can find in your object storage bucket. Just like sidecars and query nodes, the store gateway exposes StoreAPI and needs to be discovered by Thanos Querier.

```bash
thanos store \
    --data-dir             /var/thanos/store \   # Disk space for local caches
    --objstore.config-file bucket_config.yaml \  # Bucket to fetch data from
    --http-address         0.0.0.0:19191 \       # HTTP endpoint for collecting metrics on the Store Gateway
    --grpc-address         0.0.0.0:19090         # GRPC endpoint for StoreAPI
```

The store gateway occupies small amounts of disk space for caching basic information about data in the object storage. This will rarely exceed more than a few gigabytes and is used to improve restart times. It is useful but not required to preserve it across restarts.

* *[Example Kubernetes manifest]($$-https://github.com/thanos-io/kube-thanos/blob/master/manifests/thanos-store-statefulSet.yaml-testdata/not_formatted.md-$$)*

### [Compactor]($$-components/compact.md-testdata/not_formatted.md-$$)

A local Prometheus installation periodically compacts older data to improve query efficiency. Since the sidecar backs up data as soon as possible, we need a way to apply the same process to data in the object storage.

The compactor component simply scans the object storage and processes compaction where required. At the same time it is responsible for creating downsampled copies of data to speed up queries.

```bash
thanos compact \
    --data-dir             /var/thanos/compact \  # Temporary workspace for data processing
    --objstore.config-file bucket_config.yaml \   # Bucket where to apply the compacting
    --http-address         0.0.0.0:19191          # HTTP endpoint for collecting metrics on the Compactor
```

The compactor is not in the critical path of querying or data backup. It can either be run as a periodic batch job or be left running to always compact data as soon as possible. It is recommended to provide 100-300GB of local disk space for data processing.

*NOTE: The compactor must be run as a **singleton** and must not run when manually modifying data in the bucket.*

* *[Example Kubernetes manifest]($$-https://github.com/thanos-io/kube-thanos/blob/master/examples/all/manifests/thanos-compact-statefulSet.yaml-testdata/not_formatted.md-$$)*

### [Ruler/Rule]($$-components/rule.md-testdata/not_formatted.md-$$)

In case of Prometheus with Thanos sidecar does not have enough retention, or if you want to have alerts or recording rules that requires global view, Thanos has just the component for that: the [Ruler]($$-components/rule.md-testdata/not_formatted.md-$$), which does rule and alert evaluation on top of a given Thanos Querier.

<!--- TODO explain steps  --->

<img src="$$-../img/go-in-thanos.jpg-testdata/not_formatted.md-$$" class="img-fluid" alt="Go in Thanos">

<p align="center"><img src="$$-docs/img/Thanos-logo_fullmedium.png-testdata/not_formatted.md-$$" alt="Thanos Logo"></p>

<table>
<tbody>
<tr><th>Avoid 🔥[Link](../docs/something.png)</th></tr>
<tr><td>

```go
resp, err := http.Get("http://example.com/")
if err != nil {
    // handle...
}
defer runutil.CloseWithLogOnErr(logger, resp.Body, "close response")

scanner := bufio.NewScanner(resp.Body)
// If any error happens and we return in the middle of scanning
// body, we can end up with unread buffer, which
// will use memory and hold TCP connection!
for scanner.Scan() {
```

</td></tr>
<tr><th>Better 🤓</th></tr>
</tbody>
</table>

<dsada

<taasdav>
</taasdav>

## Flags

```$
usage: thanos rule [<flags>]

ruler evaluating Prometheus rules against given Query nodes, exposing Store API
and storing old blocks in bucket

Flags:
  -h, --help                     Show context-sensitive help (also try
                                 --help-long and --help-man).
      --version                  Show application version.
      --log.level=info           Log filtering level.
      --log.format=logfmt        Log format to use. Possible options: logfmt or
                                 json.
      --tracing.config-file=<file-path>
                                 Path to YAML file with tracing configuration.
                                 See format details:
                                 https://thanos.io/tip/tracing.md/#configuration
      --tracing.config=<content>
                                 Alternative to 'tracing.config-file' flag
                                 (lower priority). Content of YAML file with
                                 tracing configuration. See format details:
                                 https://thanos.io/tip/tracing.md/#configuration
      --http-address="0.0.0.0:10902"
                                 Listen host:port for HTTP endpoints.
      --http-grace-period=2m     Time to wait after an interrupt received for
                                 HTTP Server.
      --grpc-address="0.0.0.0:10901"
                                 Listen ip:port address for gRPC endpoints
                                 (StoreAPI). Make sure this address is routable
                                 from other components.
      --grpc-grace-period=2m     Time to wait after an interrupt received for
                                 GRPC Server.
      --grpc-server-tls-cert=""  TLS Certificate for gRPC server, leave blank to
                                 disable TLS
      --grpc-server-tls-key=""   TLS Key for the gRPC server, leave blank to
                                 disable TLS
      --grpc-server-tls-client-ca=""
                                 TLS CA to verify clients against. If no client
                                 CA is specified, there is no client
                                 verification on server side. (tls.NoClientCert)
      --label=<name>="<value>" ...
                                 Labels to be applied to all generated metrics
                                 (repeated). Similar to external labels for
                                 Prometheus, used to identify ruler and its
                                 blocks as unique source.
      --data-dir="data/"         data directory
      --rule-file=rules/ ...     Rule files that should be used by rule manager.
                                 Can be in glob format (repeated).
      --resend-delay=1m          Minimum amount of time to wait before resending
                                 an alert to Alertmanager.
      --eval-interval=30s        The default evaluation interval to use.
      --tsdb.block-duration=2h   Block duration for TSDB block.
      --tsdb.retention=48h       Block retention time on local disk.
      --tsdb.no-lockfile         Do not create lockfile in TSDB data directory.
                                 In any case, the lockfiles will be deleted on
                                 next startup.
      --tsdb.wal-compression     Compress the tsdb WAL.
      --alertmanagers.url=ALERTMANAGERS.URL ...
                                 Alertmanager replica URLs to push firing
                                 alerts. Ruler claims success if push to at
                                 least one alertmanager from discovered
                                 succeeds. The scheme should not be empty e.g
                                 `http` might be used. The scheme may be
                                 prefixed with 'dns+' or 'dnssrv+' to detect
                                 Alertmanager IPs through respective DNS
                                 lookups. The port defaults to 9093 or the SRV
                                 record's value. The URL path is used as a
                                 prefix for the regular Alertmanager API path.
      --alertmanagers.send-timeout=10s
                                 Timeout for sending alerts to Alertmanager
      --alertmanagers.config-file=<file-path>
                                 Path to YAML file that contains alerting
                                 configuration. See format details:
                                 https://thanos.io/tip/components/rule.md/#configuration.
                                 If defined, it takes precedence over the
                                 '--alertmanagers.url' and
                                 '--alertmanagers.send-timeout' flags.
      --alertmanagers.config=<content>
                                 Alternative to 'alertmanagers.config-file' flag
                                 (lower priority). Content of YAML file that
                                 contains alerting configuration. See format
                                 details:
                                 https://thanos.io/tip/components/rule.md/#configuration.
                                 If defined, it takes precedence over the
                                 '--alertmanagers.url' and
                                 '--alertmanagers.send-timeout' flags.
      --alertmanagers.sd-dns-interval=30s
                                 Interval between DNS resolutions of
                                 Alertmanager hosts.
      --alert.query-url=ALERT.QUERY-URL
                                 The external Thanos Query URL that would be set
                                 in all alerts 'Source' field
      --alert.label-drop=ALERT.LABEL-DROP ...
                                 Labels by name to drop before sending to
                                 alertmanager. This allows alert to be
                                 deduplicated on replica label (repeated).
                                 Similar Prometheus alert relabelling
      --web.route-prefix=""      Prefix for API and UI endpoints. This allows
                                 thanos UI to be served on a sub-path. This
                                 option is analogous to --web.route-prefix of
                                 Promethus.
      --web.external-prefix=""   Static prefix for all HTML links and redirect
                                 URLs in the UI query web interface. Actual
                                 endpoints are still served on / or the
                                 web.route-prefix. This allows thanos UI to be
                                 served behind a reverse proxy that strips a URL
                                 sub-path.
      --web.prefix-header=""     Name of HTTP request header used for dynamic
                                 prefixing of UI links and redirects. This
                                 option is ignored if web.external-prefix
                                 argument is set. Security risk: enable this
                                 option only if a reverse proxy in front of
                                 thanos is resetting the header. The
                                 --web.prefix-header=X-Forwarded-Prefix option
                                 can be useful, for example, if Thanos UI is
                                 served via Traefik reverse proxy with
                                 PathPrefixStrip option enabled, which sends the
                                 stripped prefix value in X-Forwarded-Prefix
                                 header. This allows thanos UI to be served on a
                                 sub-path.
      --log.request.decision=LogFinishCall
                                 Request Logging for logging the start and end
                                 of requests. LogFinishCall is enabled by
                                 default. LogFinishCall : Logs the finish call
                                 of the requests. LogStartAndFinishCall : Logs
                                 the start and finish call of the requests.
                                 NoLogCall : Disable request logging.
      --objstore.config-file=<file-path>
                                 Path to YAML file that contains object store
                                 configuration. See format details:
                                 https://thanos.io/tip/thanos/storage.md/#configuration
      --objstore.config=<content>
                                 Alternative to 'objstore.config-file' flag
                                 (lower priority). Content of YAML file that
                                 contains object store configuration. See format
                                 details:
                                 https://thanos.io/tip/thanos/storage.md/#configuration
      --query=<query> ...        Addresses of statically configured query API
                                 servers (repeatable). The scheme may be
                                 prefixed with 'dns+' or 'dnssrv+' to detect
                                 query API servers through respective DNS
                                 lookups.
      --query.config-file=<file-path>
                                 Path to YAML file that contains query API
                                 servers configuration. See format details:
                                 https://thanos.io/tip/components/rule.md/#configuration.
                                 If defined, it takes precedence over the
                                 '--query' and '--query.sd-files' flags.
      --query.config=<content>   Alternative to 'query.config-file' flag (lower
                                 priority). Content of YAML file that contains
                                 query API servers configuration. See format
                                 details:
                                 https://thanos.io/tip/components/rule.md/#configuration.
                                 If defined, it takes precedence over the
                                 '--query' and '--query.sd-files' flags.
      --query.sd-files=<path> ...
                                 Path to file that contains addresses of query
                                 API servers. The path can be a glob pattern
                                 (repeatable).
      --query.sd-interval=5m     Refresh interval to re-read file SD files.
                                 (used as a fallback)
      --query.sd-dns-interval=30s
                                 Interval between DNS resolutions.

```

## Configuration

### Alertmanager

The `--alertmanagers.config` and `--alertmanagers.config-file` flags allow specifying multiple Alertmanagers. Those entries are treated as a single HA group. This means that alert send failure is claimed only if the Ruler fails to send to all instances.

The configuration format is the following:

```yaml
alertmanagers:
- http_config:
    basic_auth:
      username: ""
      password: ""
      password_file: ""
    bearer_token: ""
    bearer_token_file: ""
    proxy_url: ""
    tls_config:
      ca_file: ""
      cert_file: ""
      key_file: ""
      server_name: ""
      insecure_skip_verify: false
  static_configs: []
  file_sd_configs:
  - files: []
    refresh_interval: 0s
  scheme: http
  path_prefix: ""
  timeout: 10s
  api_version: v1
```

```Makefile
include .bingo/Variables.mk
run:
	$(<PROVIDED_TOOL_NAME>) <args>
```

```Makefile
include .bingo/Variables.mk

run:
	$(<PROVIDED_TOOL_NAME>) <args>
```
