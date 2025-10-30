# Go Web Exact

[![Language](https://img.shields.io/badge/Language-Go-blue)](https://golang.org/)
[![Go Version](https://img.shields.io/github/go-mod/go-version/shouni/go-web-exact)](https://golang.org/)
[![GitHub tag (latest by date)](https://img.shields.io/github/v/tag/shouni/go-web-exact)](https://github.com/shouni/go-web-exact/tags)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

## 🚀 特徴

* **高精度なコンテンツ抽出:** 独自のセレクタとヒューリスティックを用いて、ナビゲーション、広告、コメントなどのノイズを排除し、メインの記事本文を特定します。

* **メモリ安全なリクエスト制限:** レスポンスボディの最大読み込みサイズを**25MB**に制限することで、予期せぬ巨大ファイルによる**メモリ枯渇（OOM）**を防ぎます。

* **堅牢なリトライメカニズム (Exponential Backoff):**
  リトライロジックは **`pkg/client` パッケージに統合**され、HTTPリクエストの堅牢性を高めています。`backoff/v4` を用いた**指数バックオフと自動リトライ機能**により、一時的なネットワークエラーやサーバーエラーに自動で対応します。

### 🔃 デフォルトのリトライ設定（`pkg/client` の定数に基づく）

| 設定項目 | 値 | 概要 |
| :--- | :--- |:---|
| **初期間隔** (`InitialBackoffInterval`) | **5秒** | 最初の失敗から次の試行までの待機時間。指数バックオフの開始点です。 |
| **最大間隔** (`MaxBackoffInterval`) | **30秒** | 指数バックオフにより待機時間が最大となる秒数。サーバーへの過負荷を防ぎます。 |
| **デフォルト最大試行回数** (`DefaultMaxRetries`) | **3回** | デフォルトでの最大リトライ回数。 |
| **遅延戦略** | 指数バックオフ (Jitter適用) | 待機時間を指数関数的に増加させ、ランダムな揺らぎを加えることで、リトライの集中を避けます。 |

  **注:** 最大試行回数を含む全ての設定は、**`client.New()` 関数に `client.WithMaxRetries(N)` オプションを渡す**ことで上書き可能です。

* **堅牢なHTTP処理 (GET/POST対応):** `context` を使用したタイムアウト制御に加え、**GETリクエストとJSON POSTリクエスト**の両方をサポートし、不安定なネットワーク環境や一時的なサーバーエラーに対応します。

* **型安全なエラー処理:** HTTP 4xx エラー（クライアントエラー）を非リトライ対象のカスタムエラー型で返し、5xx エラー（サーバーエラー）やネットワークエラーのみをリトライ対象とすることで、リソースの無駄遣いを防ぎます。

* **テキストの整形:** 抽出されたテキストから不要な改行や連続するスペースを除去し、クリーンな整形済みテキストを返します。

* **テーブルデータの構造化:** HTMLテーブルをパースし、Markdown風の行形式に整形してテキストに含めます。

-----

## 📦 ライブラリ利用方法

主要な機能は **`pkg/client`** と **`pkg/extract`** パッケージとして提供されます。これらは**依存性注入 (DI)** の原則に従って設計されています。

### 1\. インポート

```go
import (
    "context"
    "time"

    "[github.com/shouni/go-web-exact/pkg/client](https://github.com/shouni/go-web-exact/pkg/client)"  // HTTPクライアントパッケージ
    "[github.com/shouni/go-web-exact/pkg/extract](https://github.com/shouni/go-web-exact/pkg/extract)" // Web抽出ロジックパッケージ
)
````

### 2\. コンテンツの抽出 (`extract.Extractor` の利用)

`extract.Extractor` は `client.Client` を依存性として受け取ります。

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    "[github.com/shouni/go-web-exact/pkg/client](https://github.com/shouni/go-web-exact/pkg/client)"
    "[github.com/shouni/go-web-exact/pkg/extract](https://github.com/shouni/go-web-exact/pkg/extract)"
)

func main() {
    url := "[https://blog.golang.org/gofmt](https://blog.golang.org/gofmt)"
    
    // 1. HTTPクライアント (Fetcher) を設定
    clientTimeout := 30 * time.Second 
    
    // 2. Clientを初期化 (最大5回リトライ設定)
    fetcher := client.New(clientTimeout, client.WithMaxRetries(5)) 
    
    // 3. Extractor を初期化 (DI)
    extractor := extract.NewExtractor(fetcher) // extract.NewExtractor に変更

    // 4. 全体処理のコンテキストを設定（例：全体で60秒のタイムアウト）
    ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
    defer cancel()
    
    // 5. 抽出の実行
    text, hasBody, err := extractor.FetchAndExtractText(url, ctx)
    
    if err != nil {
       // 非リトライ対象エラーかどうか client.IsNonRetryableError(err) で判定可能
       log.Fatalf("抽出エラー: %v", err)
    }

    if !hasBody {
       fmt.Printf("本文は見つかりませんでしたが、タイトルを取得しました:\n%s\n", text)
    } else {
       fmt.Println("--- 抽出された本文 ---")
       fmt.Println(text)
       fmt.Println("-----------------------")
    }
}
```

-----

## 🛠️ 開発者向け情報

### パッケージ構成

| ディレクトリ | パッケージ名 | 役割 |
| :--- | :--- | :--- |
| **`pkg/client`** | **`client`** | HTTPリクエストの実行、カスタムエラー（`NonRetryableHTTPError`）の定義。**リトライロジックもこのパッケージに統合**されています。**`Client` 構造体は `Doer` インターフェースを満たします。** |
| **`pkg/extract`** | **`extract`** | HTMLの解析 (`goquery`)、メインコンテンツの特定、ノイズ除去、テキスト整形ロジック。 |

-----

### 📜 ライセンス (License)

このプロジェクトは [MIT License](https://opensource.org/licenses/MIT) の下で公開されています。

