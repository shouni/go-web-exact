package scraper

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/sync/errgroup"
	"golang.org/x/time/rate"

	"github.com/shouni/go-web-exact/v2/ports"
)

const (
	// DefaultMaxConcurrency は、並列スクレイピングのデフォルトの最大同時実行数を定義します。
	DefaultMaxConcurrency = 10
	// DefaultScrapeRateLimit は、ウェブスクレイピング時のデフォルトの最小リクエスト間隔 (Duration)
	// 1秒間に1リクエストを許容する安全なレートとして設定。
	DefaultScrapeRateLimit = 200 * time.Millisecond
)

// Extractor は指定された URL からコンテンツを取得し、そこからテキストを抽出するためのインターフェースです。
type Extractor interface {
	FetchAndExtractText(ctx context.Context, url string) (string, bool, error)
}

// ParallelScraper は Scraper インターフェースを実装する並列処理構造体です。
type ParallelScraper struct {
	extractor      Extractor
	maxConcurrency int
	rateLimit      time.Duration
	limiter        *rate.Limiter
}

// NewParallelScraper は ParallelScraper を初期化します。
func NewParallelScraper(opts ...Option) *ParallelScraper {
	s := &ParallelScraper{
		maxConcurrency: DefaultMaxConcurrency,
		rateLimit:      DefaultScrapeRateLimit,
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

// ScrapeInParallel は Scraper インターフェースのメソッドを実装します。
func (s *ParallelScraper) ScrapeInParallel(ctx context.Context, urls []string) []ports.URLResult {
	g, gCtx := errgroup.WithContext(ctx)
	g.SetLimit(s.maxConcurrency)
	limiter := rate.NewLimiter(rate.Every(s.rateLimit), 1)
	resultsChan := make(chan ports.URLResult, len(urls))

	for _, url := range urls {
		g.Go(func() error {
			if err := limiter.Wait(gCtx); err != nil {
				resultsChan <- ports.URLResult{URL: url, Error: err}
				return nil // グループ全体を停止させない場合
			}

			content, hasBodyFound, err := s.extractor.FetchAndExtractText(gCtx, url)

			var extractErr error
			if err != nil {
				extractErr = fmt.Errorf("コンテンツの抽出に失敗しました: %w", err)
			} else if !hasBodyFound {
				extractErr = fmt.Errorf("URL %s から有効な本文を抽出できませんでした", url)
			}

			resultsChan <- ports.URLResult{URL: url, Content: content, Error: extractErr}
			return nil
		})
	}

	_ = g.Wait()
	close(resultsChan)

	var finalResults []ports.URLResult
	for res := range resultsChan {
		finalResults = append(finalResults, res)
	}

	return finalResults
}
