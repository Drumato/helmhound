## Requirements

Please read [requirements.md](./docs/requirements.md).

## Development Cycle

If you apply any changes to this project, please run `make` to ensure that all changes are properly applied and the project is built correctly.

and, run with `./helmhound.exe --chart-url "oci://ghcr.io/prometheus-community/charts/kube-prometheus-stack" --value-path "prometheus.enabled"`.

all exposed function should be documented and tested.

## Testing strategy

All tests are written in Go table-driven style.
and all sub-testcases should be run concurrently by using `t.Parallel()`.
