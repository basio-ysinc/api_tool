# api_tool

API定義ツール群。XLSX/YAML形式のAPI定義を相互変換し、テンプレートからコード生成を行う。

## インストール

```bash
go build -o api_tool .
```

## コマンド一覧

| コマンド | 説明 |
|---------|------|
| `xlsx2yaml` | API定義の変換 xlsx -> yaml |
| `yaml2xlsx` | API定義の変換 yaml -> xlsx |
| `yaml2swagger` | API定義の変換 yaml -> swagger (OpenAPI 3.0) |
| `gen-single` | API定義からテキスト生成 (単一ファイル) |
| `gen-multiple` | API定義から複数のテキスト生成 |

---

## xlsx2yaml

XLSX形式のAPI定義をYAML形式に変換する。

```bash
api_tool xlsx2yaml <OUTPUT_PATH> INPUTS...
```

### 引数

| 引数 | 説明 |
|------|------|
| `<OUTPUT_PATH>` | 出力YAMLファイルパス |
| `INPUTS...` | 入力XLSXファイル（複数可） |

---

## yaml2xlsx

YAML形式のAPI定義をXLSX形式に変換する。

```bash
api_tool yaml2xlsx <OUTPUT_PATH> INPUTS...
```

### 引数

| 引数 | 説明 |
|------|------|
| `<OUTPUT_PATH>` | 出力XLSXファイルパス |
| `INPUTS...` | 入力YAMLファイル（複数可） |

---

## yaml2swagger

YAML/XLSX形式のAPI定義をSwagger（OpenAPI 3.0）形式に変換する。

- リクエストは全て **POST** メソッド
- パラメーターは全て **クエリストリング**

```bash
api_tool yaml2swagger [options] <OUTPUT_PATH> INPUTS...
```

### 引数

| 引数 | 説明 |
|------|------|
| `<OUTPUT_PATH>` | 出力ファイルパス |
| `INPUTS...` | 入力ファイル（yaml/xlsx） |

### オプション

| オプション | 説明 | デフォルト |
|-----------|------|-----------|
| `--only=<GROUPS>` | 出力するグループ名（カンマ区切り） | 全グループ |
| `--format=<FORMAT>` | 出力形式 `json` / `yaml` | `json` |
| `--base-path=<PATH>` | APIのベースパス | `/api` |
| `--title=<TITLE>` | APIタイトル | `API` |
| `--version=<VERSION>` | APIバージョン | `1.0.0` |

### 例

```bash
# JSON形式で出力
api_tool yaml2swagger --title="My API" --base-path=/v1 swagger.json api.yaml

# YAML形式で出力
api_tool yaml2swagger --format=yaml swagger.yaml api.yaml

# 特定グループのみ出力
api_tool yaml2swagger --only=user,auth swagger.json api.yaml
```

### 出力仕様

- OpenAPI 3.0.3 形式
- パスは `{base-path}/{group}/{actionName}/` 形式（末尾スラッシュあり、actionNameは小文字始まり）
- Enum/カスタム型は `components/schemas` に定義
- レスポンスは `{action}Response` としてスキーマ定義

---

## gen-single

API定義とpongo2テンプレートから単一のテキストファイルを生成する。

```bash
api_tool gen-single [options] <OUTPUT_PATH_PATTERN> <TEMPLATE_PATH> INPUTS...
```

### 引数

| 引数 | 説明 |
|------|------|
| `<OUTPUT_PATH_PATTERN>` | 出力ファイルパスパターン（pongo2） |
| `<TEMPLATE_PATH>` | テンプレートファイルパス |
| `INPUTS...` | 入力ファイル（xlsx/yaml） |

### オプション

| オプション | 説明 | デフォルト |
|-----------|------|-----------|
| `--only=<GROUPS>` | 出力するグループ名（カンマ区切り） | 全グループ |
| `--overwrite=<MODE>` | 上書きモード `force` / `skip` / `clear` | `force` |
| `--arg=<ARGS>` | 追加引数 `key1:value1,key2:value2` | なし |

### テンプレートコンテキスト

| 変数 | 説明 |
|------|------|
| `enums` | Enum定義リスト |
| `types` | Type定義リスト |
| `actions` | Action定義リスト |
| `groups` | Group定義リスト |

---

## gen-multiple

API定義とpongo2テンプレートから複数のテキストファイルを生成する。

```bash
api_tool gen-multiple [options] <TARGET> <OUTPUT_PATH_PATTERN> <TEMPLATE_PATH> INPUTS...
```

### 引数

| 引数 | 説明 |
|------|------|
| `<TARGET>` | 生成対象 `action` / `type` / `enum` |
| `<OUTPUT_PATH_PATTERN>` | 出力ファイルパスパターン（pongo2） |
| `<TEMPLATE_PATH>` | テンプレートファイルパス |
| `INPUTS...` | 入力ファイル（xlsx/yaml） |

### オプション

| オプション | 説明 | デフォルト |
|-----------|------|-----------|
| `--only=<GROUPS>` | 出力するグループ名（カンマ区切り） | 全グループ |
| `--overwrite=<MODE>` | 上書きモード `force` / `skip` / `clear` | `force` |
| `--arg=<ARGS>` | 追加引数 `key1:value1,key2:value2` | なし |

### テンプレートコンテキスト

`gen-single`のコンテキストに加え、ターゲットに応じた変数が追加される：

| TARGET | 追加変数 |
|--------|---------|
| `action` | `action` - 現在のAction |
| `type` | `type` - 現在のType |
| `enum` | `enum` - 現在のEnum |

---

## API定義形式

### YAML形式

```yaml
- name: グループ名
  enums:
    - name: EnumName
      description: 説明
      members:
        - name: MEMBER_NAME
          ordinal: 1
          description: 説明
  types:
    - name: TypeName
      description: 説明
      properties:
        - name: propertyName
          type: string
          description: 説明
  actions:
    - name: actionName
      description: 説明
      requestProperties:
        - name: paramName
          type: int64
          description: 説明
      responseProperties:
        - name: resultName
          type: TypeName
          description: 説明
```

### 対応する型

| 型名 | 説明 |
|------|------|
| `int`, `int8`, `int16`, `int32`, `int64` | 整数型 |
| `uint8`, `uint16`, `uint32`, `uint64` | 符号なし整数型 |
| `float`, `float32`, `float64`, `double` | 浮動小数点型 |
| `string`, `text` | 文字列型 |
| `bool`, `boolean` | 真偽値型 |
| `binary` | バイナリ型 |
| `timestamp` | タイムスタンプ型 |
| `[]TypeName` | 配列型 |
| `CustomType` | カスタム型（types で定義） |
| `EnumType` | Enum型（enums で定義） |
