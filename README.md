# Go Web Exact

[![Language](https://img.shields.io/badge/Language-Go-blue)](https://golang.org/)
[![Go Version](https://img.shields.io/github/go-mod/go-version/shouni/go-web-exact)](https://golang.org/)
[![GitHub tag (latest by date)](https://img.shields.io/github/v/tag/shouni/go-web-exact)](https://github.com/shouni/go-web-exact/tags)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

## 💡 概要 (About) — 高精度抽出と並列処理に特化した Web コンテンツ解析ライブラリ

**Go Web Exact** は、Web サイトから「真に価値のある情報」を抽出し、効率的に収集するためのパワフルなツールキットです。独自のヒューリスティック解析による**高精度なメインコンテンツ抽出**と、ターゲットへの負荷を考慮した**安全な並列スクレイピング**を統合しています。

### 🌟 コンテンツ抽出 (Core Feature)
* **高精度なメインコンテンツ特定**: 独自のセレクタとヒューリスティックを用いて、広告・ナビゲーション・コメントなどのノイズを徹底排除。DOMの出現順序を厳密に維持し、文脈を壊さずに本文を抽出します。
* **構造的な重複防止**: テキスト要素とその子孫（テーブル等）の重複を、カスタム走査ロジックによって安全に制御。二重抽出を防ぎ、クリーンなデータを保証します。
* **高度なテキスト整形**: `noiseSelectors` による不要要素の自動削除に加え、連続するスペースや改行を最適化し、AI解析や音声合成に即座に利用可能なテキストを生成します。

### 🔄 データ取得と並列処理 (Advanced Features)
* **堅牢な並列スクレイピング**: `pkg/scraper` は、**セマフォによる同時実行数制御**と**時間ベースのレートリミッター**を内蔵。サーバーへの過負荷を防ぎつつ、大量のURLを最短時間で安全に処理します。
* **フィード・インテリジェンス**: RSS/Atomフィードの解析（`gofeed` ベース）を統合。最新の記事リストからシームレスに本文抽出パイプラインへ繋げることが可能です。
* **柔軟な依存関係 (DI)**: `ports.Fetcher` インターフェースを介した依存性注入を採用。`go-http-kit` 等の外部ライブラリと組み合わせることで、リトライやSSRF対策、認証ロジックを自由にカスタマイズ可能です。

---

## 📂 プロジェクト構造 (Layout)

```text
go-web-exact/
├── extract/            # コンテンツ抽出ロジック
├── feed/               # フィード解析・リンク抽出
├── scraper/            # スクレイピング実行基盤
└── ports/              # 外部境界・インターフェース定義
```

---

### 外部依存パッケージ

本プロジェクトは、以下の主要な外部パッケージに依存しています。

* **`github.com/PuerkitoBio/goquery`**: jQueryライクな構文でのHTML要素検索。
* **`github.com/mmcdole/gofeed`**: RSS/Atomフィードのパース。
* **`golang.org/x/net/html`**: 標準ライブラリによるHTMLパース。
* **`golang.org/x/time/rate`**: 堅牢なレートリミッター制御。

-----

### 📜 ライセンス (License)

このプロジェクトは [MIT License](https://opensource.org/licenses/MIT) の下で公開されています。
