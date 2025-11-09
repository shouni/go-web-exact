package scraper

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/shouni/go-web-exact/v2/pkg/extract"
	"github.com/shouni/go-web-exact/v2/pkg/types"
)

const (
	// DefaultMaxConcurrency は、並列スクレイピングのデフォルトの最大同時実行数を定義します。
	DefaultMaxConcurrency = 6
	// DefaultScrapeRateLimit は、レートリミッターを定義します。
	DefaultScrapeRateLimit = 1000 * time.Millisecond
)

// Scraper はWebコンテンツの抽出機能を提供するインターフェースです。
type Scraper interface {
	ScrapeInParallel(ctx context.Context, urls []string) []types.URLResult
}

// ParallelScraper は Scraper インターフェースを実装する並列処理構造体です。
type ParallelScraper struct {
	extractor      *extract.Extractor
	maxConcurrency int           // 最大並列数を保持するフィールド
	rateLimit      time.Duration // レートリミッターを保持するフィールド
}

// NewParallelScraper は ParallelScraper を初期化します。
// 依存性として Extractor と、最大同時実行数を受け取ります。
func NewParallelScraper(extractor *extract.Extractor, maxConcurrency int) *ParallelScraper {
	if maxConcurrency <= 0 {
		maxConcurrency = DefaultMaxConcurrency
	}
	return &ParallelScraper{
		extractor:      extractor,
		maxConcurrency: maxConcurrency,
	}
}

// ScrapeInParallel は Scraper インターフェースのメソッドを実装します。
func (s *ParallelScraper) ScrapeInParallel(ctx context.Context, urls []string) []types.URLResult {
	var wg sync.WaitGroup
	resultsChan := make(chan types.URLResult, len(urls))

	// バッファ付きチャネルをセマフォとして使用し、同時実行数を制限する
	semaphore := make(chan struct{}, s.maxConcurrency)

	ticker := time.NewTicker(s.rateLimit)
	defer ticker.Stop()
	rateLimiter := ticker.C

	for _, url := range urls {
		wg.Add(1)

		// リソース（スロット）の確保。maxConcurrency件実行中の場合はここでブロックして待機。
		semaphore <- struct{}{}

		go func(u string) {
			defer wg.Done()

			// 処理完了後にリソース（スロット）を解放。他の待機中のGoroutineが実行可能になる。
			defer func() { <-semaphore }()

			select {
			case <-rateLimiter:
				// レートリミット間隔が経過し、リクエストが許可された
			case <-ctx.Done():
				resultsChan <- types.URLResult{
					URL:   u,
					Error: ctx.Err(),
				}
				return
			}

			content, hasBodyFound, err := s.extractor.FetchAndExtractText(ctx, u)

			var extractErr error
			if err != nil {
				extractErr = fmt.Errorf("コンテンツの抽出に失敗しました: %w", err)
			} else if !hasBodyFound {
				// 本文が見つからなかった場合を抽出失敗と判断
				extractErr = fmt.Errorf("URL %s から有効な本文を抽出できませんでした", u)
			}

			resultsChan <- types.URLResult{
				URL:     u,
				Content: content,
				Error:   extractErr,
			}
		}(url)
	}

	wg.Wait()
	close(resultsChan)

	var finalResults []types.URLResult
	for res := range resultsChan {
		finalResults = append(finalResults, res)
	}

	return finalResults
}
