## 機能要件

### 基本動作

- Helm chartのURLとバージョンを指定して、Helm chartをダウンロードする
  - すでに同じバージョンのHelm chartがローカルに存在する場合は、再ダウンロードしない
- Helm chartのvalues.yamlを取得する
- ユーザはfuzzyfinderを利用して、valueを選択する
- chart内のすべてのtemplateファイルを解析し、選択したvalueが参照されている箇所を表示する

出力例を以下に示す。

```shell
$ helmhound --chart-url https://example.com/charts/mychart --chart-version 1.0.0

... (省略、ユーザはfoo.bar valueを選択した) ...

=== group/version,kind=MyResource ===
13L: .spec.foo.bar
=== group/version,kind=MyResource2 ===
25L: .spec.foo.bar
```

### CLI

- `--chart-url` でHelm chartのURLを指定できる
- `--chart-version` でHelm chartのバージョンを指定できる