![helmhound.png](./helmhound.png)

# helmhound

[English](./README.md)

Helm Chartのvaluesが生成されるマニフェストのどこに影響を与えるのかを解析するツールです。

## 概要

helmhoundは、Helmチャートの値（values）を対話的に選択し、その値を変更した場合の影響を比較・可視化するためのCLIツールです。

## 主な機能

- **対話的な値選択**: fzfを使用してHelmチャートの値パスを検索・選択
- **値の影響分析**: 選択した値を変更した場合のKubernetesマニフェストへの影響を表示
- **詳細な差分表示**: YAML構造の変更を見やすい形式で表示
- **チャートキャッシュ**: ダウンロードしたチャートをローカルにキャッシュして高速化

## インストール

### 前提条件

- Go 1.24.5以上
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

## 使用方法

### 基本的な使用方法

```bash
./helmhound.exe --chart-url "oci://ghcr.io/prometheus-community/charts/kube-prometheus-stack" --chart-version "75.17.1"
```

### 特定の値パスを直接指定

```bash
./helmhound.exe --chart-url "oci://ghcr.io/prometheus-community/charts/kube-prometheus-stack" --chart-version "75.17.1" --value-path "prometheus.enabled"
```

### required valueを持つチャートへの対応

対象のHelm Chartがrequired valueを使っており、デフォルトのvaluesだとレンダリングエラーを起こす際は、`--values-file`を使ってoverrideしてください：

```bash
./helmhound.exe --chart-url "oci://example.com/chart-with-required-values" --chart-version "1.0.0" --values-file "custom-values.yaml"
```

### ログレベルを指定

```bash
./helmhound.exe --chart-url "oci://example.com/chart" --chart-version "1.0.0" --log-level "debug"
```

### キャッシュ管理

```bash
# キャッシュされたチャートの一覧表示
./helmhound.exe cache list
```

## コマンドラインオプション

| オプション | 説明 | 必須 | デフォルト値 |
|-----------|------|------|------------|
| `--chart-url` | HelmチャートのURL | ✓ | - |
| `--chart-version` | Helmチャートのバージョン | ✓ | - |
| `--value-path` | 特定の値パス（対話選択をスキップ） | - | - |
| `--log-level` | ログレベル（debug, info, warn, error） | - | info |

## 動作の流れ

1. **チャートダウンロード**: 指定されたURLとバージョンでHelmチャートをダウンロード
2. **値の抽出**: チャートからすべての設定可能な値パスを抽出
3. **値の選択**: fzfを使用して対話的に値パスを選択（または`--value-path`で直接指定）
4. **テンプレート生成**: 
   - オリジナルの設定でKubernetesマニフェストを生成
   - 選択した値を変更した設定でマニフェストを生成
5. **差分比較**: 2つのマニフェスト間の詳細な差分を計算・表示

## 出力例

```
Selected value path: prometheus.enabled

Differences found (3 paths):
apps/v1/Deployment/monitoring/kube-prometheus-stack-prometheus:
  - spec.replicas
  - spec.template.spec.containers[0].image

v1/Service/monitoring/kube-prometheus-stack-prometheus:
  - spec.ports[0].port
```

## アーキテクチャ

### パッケージ構成

- `cmd/`: コマンドライン処理とメインロジック
- `pkg/helmwrap/`: Helm操作のラッパー
- `pkg/yamldiff/`: YAML差分計算ライブラリ

### 主要コンポーネント

#### Helm操作 (`pkg/helmwrap`)

- **Client**: Helmとの統合インターフェース
- **チャートダウンロード**: OCI/HTTPレジストリからのチャート取得
- **値抽出**: YAML構造からの設定可能パス抽出
- **テンプレートレンダリング**: Kubernetesマニフェストの生成

#### YAML差分 (`pkg/yamldiff`)

- **構造比較**: 深い階層のYAML構造比較
- **型安全**: 異なるデータ型の適切な処理
- **詳細表示**: 変更箇所の詳細な特定と表示

