# go-web-exact

[![Language](https://img.shields.io/badge/Language-Go-blue)](https://golang.org/)
[![Go Version](https://img.shields.io/github/go-mod/go-version/shouni/go-web-exact)](https://golang.org/)
[![GitHub tag (latest by date)](https://img.shields.io/github/v/tag/shouni/go-web-exact)](https://github.com/shouni/go-web-exact/tags)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

## 🎯 概要: ウェブからLLM入力用のクリーンな記事本文を抽出するCLIツール

`go-web-exact` は、Go言語で実装された、ウェブページから**ノイズを除去**し、記事の本文や主要なコンテンツを**正確に抽出**するためのCLIツール、およびコアパッケージ群です。

特に、**LLM（大規模言語モデル）に入力するためのクリーンで構造化されたテキストデータを生成する**ことを目的としています。

-----

## 🚀 特徴

* **高精度なコンテンツ抽出:** 独自のセレクタとヒューリスティックを用いて、ナビゲーション、広告、コメントなどのノイズを排除し、メインの記事本文を特定します。
* **汎用的なリトライメカニズム:** ネットワークの堅牢性を高めるため、リトライロジックを`pkg/retry`パッケージとして分離しました。HTTPクライアントやその他のAPIクライアントは、この汎用サービスを利用して、指数バックオフ (`backoff/v4`) を用いた**自動リトライ機能**を実現します。
* **堅牢なHTTP処理 (GET/POST対応):** `context` を使用したタイムアウト制御に加え、**GETリクエストとJSON POSTリクエスト**の両方をサポートし、不安定なネットワーク環境や一時的なサーバーエラーに対応します。
* **型安全なエラー処理:** HTTP 4xx エラー（クライアントエラー）を非リトライ対象のカスタムエラー型で返し、5xx エラー（サーバーエラー）やネットワークエラーのみをリトライ対象とすることで、リソースの無駄遣いを防ぎます。
* **テキストの整形:** 抽出されたテキストから不要な改行や連続するスペースを除去し、クリーンな整形済みテキストを返します。
* **テーブルデータの構造化:** HTMLテーブルをパースし、Markdown風の行形式に整形してテキストに含めます。

-----

## ⚙️ CLIとしての利用

このプロジェクトは、`cmd/root.go` をエントリーポイントとする実行可能なCLIアプリケーションとして設計されています。

### ビルドと実行

プロジェクトルートで以下のコマンドを実行し、バイナリを生成します。

```bash
# バイナリのビルド
go build -o bin/web_extractor
```

### 使用方法

生成された `web_extractor` を使用して、URLを指定します。

```bash
# 位置引数としてURLを指定
./bin/web_extractor https://example.com/article/123

# フラグとしてURLを指定し、タイムアウトを15秒に設定
./bin/web_extractor -u https://example.com/article/123 -t 15
```

**ヘルプメッセージ:**

```bash
./bin/web_extractor --help
```

-----

## 📦 ライブラリ利用方法

主要な機能は `pkg/httpclient`、`pkg/retry`、`pkg/web` パッケージとして提供されます。これらは**依存性注入 (DI)** の原則に従って設計されています。

### 1\. インポート

```go
import (
    "context"
    "time"

    "github.com/shouni/go-web-exact/pkg/httpclient" 
    "github.com/shouni/go-web-exact/pkg/web"      
    // リトライは httpclient 内部で利用されるため、通常は直接インポート不要
)
```

### 2\. コンテンツの抽出 (`web.Extractor` の利用)

`web.Extractor` は `httpclient.Client` を依存性として受け取ります。

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    "github.com/shouni/go-web-exact/pkg/httpclient" 
    "github.com/shouni/go-web-exact/pkg/web"
)

func main() {
    url := "https://blog.golang.org/gofmt"
    
    // 1. HTTPクライアント (Fetcher) を設定
    // 個々のリクエストのタイムアウトを30秒に設定
    clientTimeout := 30 * time.Second 
    fetcher := httpclient.New(clientTimeout).WithMaxRetries(5) // 最大5回のリトライを設定
    
    // 2. Extractor を初期化 (DI)
    extractor := web.NewExtractor(fetcher)

    // 3. 全体処理のコンテキストを設定（例：全体で60秒のタイムアウト）
    ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
    defer cancel()
    
    // 4. 抽出の実行
    text, hasBody, err := extractor.FetchAndExtractText(url, ctx)
    
    if err != nil {
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
| `cmd/` | `main` | CLIのエントリーポイントおよびコマンドラインオプションの定義。 |
| `pkg/httpclient` | `httpclient` | HTTPリクエストの実行、カスタムエラー（`NonRetryableHTTPError`）の定義。**`pkg/retry`** を利用して堅牢性を確保。`web.Fetcher` インターフェースを実装。 |
| `pkg/retry` | `retry` | **汎用的なリトライ実行ロジック**（`backoff/v4` の設定、指数バックオフ、最大試行回数制御）を抽象化し、提供。 |
| `pkg/web` | `web` | HTMLの解析 (`goquery`)、メインコンテンツの特定、ノイズ除去、テキスト整形ロジック。 |

-----

### 📜 ライセンス (License)

このプロジェクトは [MIT License](https://opensource.org/licenses/MIT) の下で公開されています。
