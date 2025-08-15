# helmhound

[日本語](./README.ja.md)

A tool for analyzing how Helm chart values affect generated manifests.

## Overview

helmhound is a CLI tool for interactively selecting Helm chart values and visualizing the impact of changing those values by comparing and displaying differences in rendered Kubernetes manifests.

## Key Features

- **Interactive value selection**: Search and select Helm chart value paths using fzf
- **Impact analysis**: Display the impact on Kubernetes manifests when changing selected values
- **Detailed diff display**: Show YAML structure changes in a readable format
- **Chart caching**: Cache downloaded charts locally for improved performance

## Installation

### Prerequisites

- Go 1.24.5 or higher
- [fzf](https://github.com/junegunn/fzf) - fuzzyfinder

### Via Homebrew

```bash
brew tap Drumato/formulas
brew install helmhound
```

### Build from source

```bash
git clone https://github.com/Drumato/helmhound
cd helmhound
make
```

## Usage

### Basic Usage

```bash
./helmhound.exe --chart-url "oci://ghcr.io/prometheus-community/charts/kube-prometheus-stack" --chart-version "75.17.1"
```

### Direct Value Path Specification

```bash
./helmhound.exe --chart-url "oci://ghcr.io/prometheus-community/charts/kube-prometheus-stack" --chart-version "75.17.1" --value-path "prometheus.enabled"
```

### Handling Charts with Required Values

When the target Helm Chart uses required values and causes rendering errors with default values, use `--values-file` to override them:

```bash
./helmhound.exe --chart-url "oci://example.com/chart-with-required-values" --chart-version "1.0.0" --values-file "custom-values.yaml"
```

### With Log Level

```bash
./helmhound.exe --chart-url "oci://example.com/chart" --chart-version "1.0.0" --log-level "debug"
```

### Cache Management

```bash
# List cached charts
./helmhound.exe cache list
```

## Command Line Options

| Option | Description | Required | Default |
|--------|-------------|----------|---------|
| `--chart-url` | URL of the Helm chart | ✓ | - |
| `--chart-version` | Version of the Helm chart | ✓ | - |
| `--value-path` | Specific value path (skip interactive selection) | - | - |
| `--log-level` | Log level (debug, info, warn, error) | - | info |

## How It Works

1. **Chart Download**: Downloads the Helm chart from the specified URL and version
2. **Value Extraction**: Extracts all configurable value paths from the chart
3. **Value Selection**: Interactively select a value path using fzf (or specify directly with `--value-path`)
4. **Template Rendering**: 
   - Generate Kubernetes manifests with original configuration
   - Generate manifests with the selected value modified
5. **Diff Comparison**: Calculate and display detailed differences between the two manifests

## Sample Output

```
Selected value path: prometheus.enabled

Differences found (3 paths):
apps/v1/Deployment/monitoring/kube-prometheus-stack-prometheus:
  - spec.replicas
  - spec.template.spec.containers[0].image

v1/Service/monitoring/kube-prometheus-stack-prometheus:
  - spec.ports[0].port
```

## Architecture

### Package Structure

- `cmd/`: Command-line processing and main logic
- `pkg/helmwrap/`: Helm operations wrapper
- `pkg/yamldiff/`: YAML diff calculation library

### Key Components

#### Helm Operations (`pkg/helmwrap`)

- **Client**: Integration interface with Helm
- **Chart Download**: Chart retrieval from OCI/HTTP registries
- **Value Extraction**: Extract configurable paths from YAML structures
- **Template Rendering**: Generate Kubernetes manifests

#### YAML Diff (`pkg/yamldiff`)

- **Structure Comparison**: Deep hierarchical YAML structure comparison
- **Type Safety**: Proper handling of different data types
- **Detailed Display**: Precise identification and display of changes
