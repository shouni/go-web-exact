package scraper

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/shouni/go-web-exact/v2/pkg/extract"
	"github.com/shouni/go-web-exact/v2/pkg/types"
	"golang.org/x/time/rate"
)

const (
	// DefaultMaxConcurrency は、並列スクレイピングのデフォルトの最大同時実行数を定義します。
	DefaultMaxConcurrency = 10
	// DefaultScrapeRateLimit は、ウェブスクレイピング時のデフォルトの最小リクエスト間隔 (Duration)
	// 1秒間に1リクエストを許容する安全なレートとして設定。
	DefaultScrapeRateLimit = 500 * time.Millisecond
)

// Scraper はWebコンテンツの抽出機能を提供するインターフェースです。
type Scraper interface {
	ScrapeInParallel(ctx context.Context, urls []string) []types.URLResult
}

// ParallelScraper は Scraper インターフェースを実装する並列処理構造体です。
type ParallelScraper struct {
	extractor      *extract.Extractor
	maxConcurrency int           // 最大並列数 (セマフォで使用)
	limiter        *rate.Limiter // レートリミッター (時間制御に使用)
}

// NewParallelScraper は ParallelScraper を初期化します。
// 依存性として Extractor と、最大同時実行数、レートリミット間隔を受け取ります。
func NewParallelScraper(extractor *extract.Extractor, maxConcurrency int, rateLimit time.Duration) *ParallelScraper {
	if maxConcurrency <= 0 {
		maxConcurrency = DefaultMaxConcurrency
	}
	if rateLimit <= 0 {
		rateLimit = DefaultScrapeRateLimit
	}

	// rate.Every(rateLimit) は、その期間ごとに1トークンを生成するレートを設定します。
	// バーストサイズ1は、厳密なレート制御（事前にトークンを貯めない）を意味します。
	limiter := rate.NewLimiter(rate.Every(rateLimit), 1)

	return &ParallelScraper{
		extractor:      extractor,
		maxConcurrency: maxConcurrency,
		limiter:        limiter,
	}
}

// ScrapeInParallel は Scraper インターフェースのメソッドを実装します。
func (s *ParallelScraper) ScrapeInParallel(ctx context.Context, urls []string) []types.URLResult {
	var wg sync.WaitGroup
	resultsChan := make(chan types.URLResult, len(urls))

	// バッファ付きチャネルをセマフォとして使用し、同時実行数を制限する
	semaphore := make(chan struct{}, s.maxConcurrency)

	for _, url := range urls {
		wg.Add(1)

		// リソース（スロット）の確保。maxConcurrency件実行中の場合はここでブロックして待機。
		semaphore <- struct{}{}

		go func(u string) {
			defer wg.Done()

			// 処理完了後にリソース（スロット）を解放。他の待機中のGoroutineが実行可能になる。
			defer func() { <-semaphore }()

			// rate.Limiter.Wait() を使用して、レート制限の待機とコンテキストキャンセルを同時に処理
			// Wait(ctx) は、レートリミットに達した場合に待機し、ctx.Done() が発火した場合はエラーを返す。
			if err := s.limiter.Wait(ctx); err != nil {
				// レートリミット待機中にコンテキストがキャンセルされた場合のエラー処理
				resultsChan <- types.URLResult{
					URL:   u,
					Error: fmt.Errorf("レートリミット待機中にキャンセル: %w", err),
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
