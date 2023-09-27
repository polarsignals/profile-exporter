![Build](https://github.com/polarsignals/profile-exporter/actions/workflows/build.yml/badge.svg)
![Container](https://github.com/polarsignals/profile-exporter/actions/workflows/container.yml/badge.svg)
[![Apache 2 License](https://img.shields.io/badge/license-Apache%202-blue.svg)](LICENSE)

# profile-exporter

With a given configuration `profile-exporter` queries a [Parca](https://parca.dev/) compatible API, transforms the result into metrics and sends the metrics to a [Prometheus remote write endpoint](https://prometheus.io/docs/concepts/remote_write_spec/).

It produces the follow metrics:

1) `profile_exporter_root_cumulative_value` (labels: `query`, `query_name`): The total value as it would be the root in a flamegraph.
2) `profile_exporter_cumulative_value` (labels: `query`, `query_name`, `function_name`): The value of the function plus all the functions it called.
3) `profile_exporter_flat_value` (labels: `query`, `query_name`, `function_name`): The value that only the function used itself.

## Configuration

`profile-exporter` is configured by a single configuration file passed via the `--config-file` flag (defaults to `profile-exporter.yaml`). See [`profile-exporter.yaml`](profile-exporter.yaml) as an example.

```yaml
remote_write:
  # The URL of the endpoint to send samples to.
  url: <string>

  # Timeout for requests to the remote write endpoint.
  [ remote_timeout: <duration | default = 30s> ]

  # All Prometheus HTTP client configuration options,
  # sigv4 and Azure AD authentication options are available.

parca:
  # gRPC endpoint without protocol
  address: <string>
  # bearer token to use for authentication, `bearerTokenFile` is recommended
  bearerToken: <string>
  # file to read bearer token from for authentication
  bearerTokenFile: <string>
  # connect to Parca-compatible API via an insecure connection
  insecure: <bool>
  # ignore verification of TLS certificate used by Parca-compatible API
  insecure_skip_verify: <bool>

queries:
  # Name of query (will be a label in metrics).
- name: <string>
  # Query to run against Parca-compatible API.
  query: <string>
  # Duration to run query for, as in "last 5m".
  duration: <string>
  # Which functions to generate metrics for.
  matchers:
    # Function name contains the substring
  - contains: <string>
```

## Demo

1) Run Prometheus with the `--web.enable-remote-write-receiver` flag.
2) Run Parca.
3) Run profile-exporter.

## Configuration

Flags:

[embedmd]:# (dist/help.txt)
```txt
Usage: profile-exporter

Flags:
  -h, --help                Show context-sensitive help.
      --log-level="info"    Log level.
      --config-file="profile-exporter.yaml"
                            Path to the config file.
```
