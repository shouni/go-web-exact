# Go Web Exact

[![Language](https://img.shields.io/badge/Language-Go-blue)](https://golang.org/)
[![Go Version](https://img.shields.io/github/go-mod/go-version/shouni/go-web-exact)](https://golang.org/)
[![GitHub tag (latest by date)](https://img.shields.io/github/v/tag/shouni/go-web-exact)](https://github.com/shouni/go-web-exact/tags)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

このプロジェクトは、Webコンテンツの**高精度なメインコンテンツ抽出**、**フィード解析**、および**並列スクレイピング**に特化した多機能ツール/ライブラリです。

-----

## 🚀 主な機能と特徴

### 🌟 コンテンツ抽出 (**Core Feature**)

* **高精度なコンテンツ抽出:** 独自のセレクタとヒューリスティックを用いて、ナビゲーション、広告、コメントなどの**ノイズを排除**し、メインの記事本文を特定します。HTMLの**DOM出現順序を厳密に維持**してテキストを結合します。
* **コンテンツ重複の自動防止:** 一般的なテキスト要素の子孫に含まれるテーブルなどを、**安全なカスタム走査ロジック**によって自動的に除外し、重複抽出を防ぎます。
* **ノイズの排除:** 抽出ロジック内で定義された **`noiseSelectors`** を使用して、指定された不要な要素を削除します。
* **テキストの整形:** 抽出されたテキストから不要な改行や連続するスペースを除去し、クリーンな整形済みテキストを返します。

### 🔄 データ取得と並列処理 (**New Features**)

* **並列スクレイピング (`pkg/scraper`):** 複数のURLに対するコンテンツ抽出処理を、**セマフォ制御**により指定された最大同時実行数で安全に**並列実行**します。大量のコンテンツを効率的に処理できます。
* **フィード解析 (`pkg/feed`):** RSS/Atomフィードの取得と解析（`gofeed` に依存）を提供し、フィード内の記事情報を抽出します。
* **柔軟な依存関係 (DI):** HTTPリクエストの実行には、外部で定義された **`extract.Fetcher` インターフェース**を依存性注入（DI）により受け取ります。これにより、リトライ、認証、キャッシュなどのロジックをアプリケーション側で自由に実装・選択できます。（例: `go-http-kit` の利用）

-----

## 📦 ライブラリ利用方法

### 1\. 単一URLのコンテンツ抽出 (`pkg/extract`)

主要な抽出機能は **`pkg/extract`** パッケージとして提供されます。外部のHTTPクライアントは `extract.Fetcher` インターフェースを満たす必要があります。

#### 1-1. インターフェース定義 (Fetcher)

`go-web-exact` は、以下の **`Fetcher`** インターフェースに依存します。

```go
package extract

import "context"

// Fetcher は、指定されたURLからリトライ付きでコンテンツを取得するクライアントインターフェースです。
type Fetcher interface {
    FetchBytes(ctx context.Context, url string) ([]byte, error)
}
```

#### 1-2. コンテンツの抽出 (`extract.Extractor` の利用)

```go
package main

// ... 必要な import ...

func main() {
    // ... Fetcher の初期化 ... 
    
    // 2. Extractor を初期化 (FetcherをDI)
    extractor, err := extract.NewExtractor(fetcher)
    // ... (エラー処理) ...

    // 4. 抽出の実行
    text, hasBody, err := extractor.FetchAndExtractText(ctx, url)

    // ... (結果の出力) ...
}
```

### 2\. 複数のURLの並列抽出 (`pkg/scraper`)

`pkg/scraper` パッケージは、`extract.Extractor` を利用して複数のURLを効率的に処理します。

```go
package main

// ... 必要な import ...
import "github.com/shouni/go-web-exact/v2/pkg/scraper"

func main() {
    // ... Extractor の初期化 ... 
    
    urlsToScrape := []string{"url1", "url2", "url3"}
    
    // 1. ParallelScraperを初期化 (最大同時実行数: 5)
    maxConcurrency := 5
    parallelScraper := scraper.NewParallelScraper(extractor, maxConcurrency)

    // 2. 並列抽出を実行
    results := parallelScraper.ScrapeInParallel(context.Background(), urlsToScrape)
    
    // 3. 結果の処理 (results は []types.URLResult です)
    for _, res := range results {
        if res.Error != nil {
            fmt.Printf("❌ 失敗: %s, エラー: %v\n", res.URL, res.Error)
        } else {
            fmt.Printf("✅ 成功: %s, 長さ: %d\n", res.URL, len(res.Content))
        }
    }
}
```

-----

## 🛠️ 開発者向け情報

### パッケージ構成

| ディレクトリ | パッケージ名 | 役割 |
| :--- | :--- | :--- |
| **`pkg/extract`** | **`extract`** | HTML解析、メインコンテンツ特定、ノイズ除去、テキスト整形ロジック。 |
| **`pkg/feed`** | **`feed`** | RSS/Atomフィードの取得、パース、データ構造化ロジック。 |
| **`pkg/scraper`** | **`scraper`** | `extract` パッケージを利用した複数URLの**並列処理**制御ロジック（セマフォ制御）。 |
| **`pkg/types`** | **`types`** | アプリケーション全体で共有されるデータ構造 (`URLResult`, `TemplateData` など) の定義。 |

### 外部依存パッケージ

本プロジェクトは、以下の主要な外部パッケージに依存しています。

* **`github.com/PuerkitoBio/goquery`**: jQueryライクな構文でのHTML要素検索。
* **`github.com/mmcdole/gofeed`**: RSS/Atomフィードのパース。
* **`golang.org/x/net/html`**: 標準ライブラリによるHTMLパース。

-----

### 📜 ライセンス (License)

このプロジェクトは [MIT License](https://opensource.org/licenses/MIT) の下で公開されています。
