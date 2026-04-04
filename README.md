# Go Web Exact

[![Language](https://img.shields.io/badge/Language-Go-blue)](https://golang.org/)
[![Go Version](https://img.shields.io/github/go-mod/go-version/shouni/go-web-exact)](https://golang.org/)
[![GitHub tag (latest by date)](https://img.shields.io/github/v/tag/shouni/go-web-exact)](https://github.com/shouni/go-web-exact/tags)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

## 💡 概要 (About) — 高精度抽出と堅牢な実行パイプラインに特化した Web 解析ライブラリ

**Go Web Exact** は、Web サイトから「真に価値のある情報」を抽出し、確実に収集するためのパワフルなツールキットです。
独自のヒューリスティック解析による**高精度なメインコンテンツ抽出**に加え、レート制限付きの**並列処理**と、失敗を補完する**自動リトライ戦略**を統合しています。

### 🌟 コンテンツ抽出 (Core Extraction)

* **高精度なメインコンテンツ特定**: 独自のセレクタとヒューリスティックを用いて、広告・ナビゲーション・コメントなどのノイズを徹底排除。DOMの出現順序を維持し、文脈を壊さずに本文を抽出します。
* **構造的な重複防止**: テキスト要素とその子孫の重複を、カスタム走査ロジックによって安全に制御。クリーンなデータを保証します。
* **高度なテキスト整形**: 連続するスペースや改行の最適化を行い、AI解析やLLMプロンプトに即座に利用可能なテキストを生成します。

### 🔄 実行オーケストレーション (Orchestration)

* **Concurrent Scraper**: `scraper` パッケージは、**errgroup による同時実行数制御**と **token bucket アルゴリズムによるレート制限**を内蔵。ターゲットサーバーへの負荷を抑えつつ、大量のURLを最短時間で安全に処理します。
* **Robust Runner**: `runner` パッケージは、並列処理での取りこぼし（一時的なエラーや本文未検出）を検知し、適切なディレイを挟んで**逐次リトライ**を実行。データの欠損を最小限に抑えます。
* **Dependency Injection**: `ports` インターフェースを介した疎結合な設計を採用。`builder` パッケージにより、設定に応じた最適なインスタンス構築を容易に行えます。

-----

## 📂 プロジェクト構造 (Layout)

```text
go-web-exact/
├── extract/    # 単一URLからのコンテンツ抽出 (Extractor)
├── scraper/    # 並列実行・レート制限エンジン (Concurrent)
├── runner/     # リトライ・フェーズ管理等の実行戦略 (Runner)
├── builder/    # 依存関係の組み立て・インスタンス生成 (Builder)
└── ports/      # 共通インターフェース・データ構造の定義
```

-----

### 外部依存パッケージ

本プロジェクトは、以下の主要な外部パッケージに依存しています。

* **`github.com/PuerkitoBio/goquery`**: HTML要素の走査と検索。
* **`golang.org/x/sync/errgroup`**: 高度な並列処理制御。
* **`golang.org/x/time/rate`**: 堅牢なレートリミッター。

-----

### 📜 ライセンス (License)

このプロジェクトは [MIT License](https://opensource.org/licenses/MIT) の下で公開されています。

-----