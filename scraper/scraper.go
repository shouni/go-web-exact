package scraper

import (
	"context"
	"fmt"
	"time"

	"github.com/shouni/go-web-exact/v2/extract"
	"github.com/shouni/go-web-exact/v2/types"
	"golang.org/x/sync/errgroup"
	"golang.org/x/time/rate"
)

const (
	// DefaultMaxConcurrency は、並列スクレイピングのデフォルトの最大同時実行数を定義します。
	DefaultMaxConcurrency = 10
	// DefaultScrapeRateLimit は、ウェブスクレイピング時のデフォルトの最小リクエスト間隔 (Duration)
	// 1秒間に1リクエストを許容する安全なレートとして設定。
	DefaultScrapeRateLimit = 200 * time.Millisecond
)

// Scraper はWebコンテンツの抽出機能を提供するインターフェースです。
type Scraper interface {
	ScrapeInParallel(ctx context.Context, urls []string) []types.URLResult
}

// Extractor は指定された URL からコンテンツを取得し、そこからテキストを抽出するためのインターフェースです。
type Extractor interface {
	FetchAndExtractText(ctx context.Context, url string) (string, bool, error)
}

// ParallelScraper は Scraper インターフェースを実装する並列処理構造体です。
type ParallelScraper struct {
	extractor      Extractor
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
	g, gCtx := errgroup.WithContext(ctx)
	g.SetLimit(s.maxConcurrency)

	resultsChan := make(chan types.URLResult, len(urls))

	for _, url := range urls {
		g.Go(func() error {
			if err := s.limiter.Wait(gCtx); err != nil {
				resultsChan <- types.URLResult{URL: url, Error: err}
				return nil // グループ全体を停止させない場合
			}

			content, hasBodyFound, err := s.extractor.FetchAndExtractText(gCtx, url)

			var extractErr error
			if err != nil {
				extractErr = fmt.Errorf("コンテンツの抽出に失敗しました: %w", err)
			} else if !hasBodyFound {
				extractErr = fmt.Errorf("URL %s から有効な本文を抽出できませんでした", url)
			}

			resultsChan <- types.URLResult{URL: url, Content: content, Error: extractErr}
			return nil
		})
	}

	_ = g.Wait()
	close(resultsChan)

	var finalResults []types.URLResult
	for res := range resultsChan {
		finalResults = append(finalResults, res)
	}

	return finalResults
}
