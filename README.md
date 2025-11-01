# Go Web Exact

[![Language](https://img.shields.io/badge/Language-Go-blue)](https://golang.org/)
[![Go Version](https://img.shields.io/github/go-mod/go-version/shouni/go-web-exact)](https://golang.org/)
[![GitHub tag (latest by date)](https://img.shields.io/github/v/tag/shouni/go-web-exact)](https://github.com/shouni/go-web-exact/tags)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

## 🚀 特徴

このライブラリは、Webコンテンツの取得を外部の **`Fetcher` インターフェース**に依存し、HTMLドキュメントからの**高精度なメインコンテンツ抽出**に特化しています。

* **高精度なコンテンツ抽出:** 独自のセレクタとヒューリスティックを用いて、ナビゲーション、広告、コメントなどの**ノイズを排除**し、メインの記事本文を特定します。

* **柔軟な依存関係 (DI):** HTTPリクエストの実行には、外部で定義された **`extract.Fetcher` インターフェース**を依存性注入（DI）により受け取ります。これにより、リトライ、認証、キャッシュなどのロジックをアプリケーション側で自由に実装・選択できます。（例: `go-http-kit` の利用）

* **ノイズの排除:** 抽出ロジック内で定義された **`noiseSelectors`** を使用して、指定された不要な要素を削除します。

* **テキストの整形:** 抽出されたテキストから不要な改行や連続するスペースを除去し、クリーンな整形済みテキストを返します。

* **テーブルデータの構造化:** HTMLテーブルをパースし、Markdown風の行形式に整形してテキストに含めます。

* **抽出ルールの公開:** 最小段落長（20文字）、最小見出し長（3文字）など、抽出に使用する具体的なルールを公開しています。

-----

## 📦 ライブラリ利用方法

主要な機能は **`pkg/extract`** パッケージとして提供されます。外部のHTTPクライアントは **`extract.Fetcher`** インターフェースを満たす必要があります。

### 1\. インターフェース定義 (Fetcher)

`go-web-exact` は、以下の **`Fetcher`** インターフェースに依存します。

```go
// pkg/extract/interface.go (このライブラリ内で定義)

// Fetcher は、指定されたURLからリトライ付きでコンテンツを取得するクライアントインターフェースです。
type Fetcher interface {
    FetchBytes(url string, ctx context.Context) ([]byte, error)
}
````

### 2\. コンテンツの抽出 (`extract.Extractor` の利用)

`extract.Extractor` は `Fetcher` 実装（例: `go-http-kit` で実装されたクライアント）を依存性として受け取ります。

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"

	"github.com/shouni/go-http-kit/pkg/httpkit" 
    "github.com/shouni/go-web-exact/v2/pkg/extract"
)

// main関数の例
func main() {
    url := "[https://github.com/shouni/go-web-exact](https://github.com/shouni/go-web-exact)"

    // 1. 外部の Fetcher 実装を初期化 (go-http-kitを利用)
    // httpkit.Client は extract.Fetcher インターフェースを満たします
    clientTimeout := 30 * time.Second
    fetcher := httpkit.New(clientTimeout, httpkit.WithMaxRetries(5))

    // 2. Extractor を初期化 (FetcherをDI)
    extractor, err := extract.NewExtractor(fetcher)

    // 3. 全体処理のコンテキストを設定
    ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
    defer cancel()

    // 4. 抽出の実行
    text, hasBody, err := extractor.FetchAndExtractText(url, ctx)

    if err != nil {
       // エラー処理 
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
| **`pkg/extract`** | **`extract`** | HTMLの解析 (`goquery`)、メインコンテンツの特定、ノイズ除去、テキスト整形ロジック。 |
| **`pkg/extract/interface.go`** | **`extract`** | 外部依存となる **`Fetcher`** インターフェースの定義。 |

### 外部依存パッケージ

本プロジェクトは、以下の主要な外部パッケージに依存しています。

* **`github.com/antchfx/htmlquery`**: HTMLドキュメントの操作。
* **`github.com/PuerkitoBio/goquery`**: jQueryライクな構文でのHTML要素検索。
* **`golang.org/x/net/html`**: 標準ライブラリによるHTMLパース。

-----

### 📜 ライセンス (License)

このプロジェクトは [MIT License](https://opensource.org/licenses/MIT) の下で公開されています。


