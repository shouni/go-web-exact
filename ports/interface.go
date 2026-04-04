package ports

import (
	"context"
)

// Fetcher は、HTMLドキュメントの生バイト配列を取得する機能のインターフェースを定義します。
type Fetcher interface {
	FetchBytes(ctx context.Context, url string) ([]byte, error)
}

// Extractor は指定された URL からコンテンツを取得し、そこからテキストを抽出するためのインターフェースです。
type Extractor interface {
	FetchAndExtractText(ctx context.Context, url string) (string, bool, error)
}

// Scraper はWebコンテンツの抽出機能を提供するインターフェースです。
type Scraper interface {
	Run(ctx context.Context, urls []string) []URLResult
}

// ScrapeRunner は、スクレイピングの実行パイプライン（並列処理、リトライ制御など）を管理するインターフェースです。
type ScrapeRunner interface {
	Run(ctx context.Context, urls []string) []URLResult
}
