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
	// DefaultRateLimit は、ウェブスクレイピング時のデフォルトの最小リクエスト間隔 (Duration)
	DefaultRateLimit = 200 * time.Millisecond
)

// Concurrent は、並列かつレート制限を考慮してスクレイピングを実行するエンジンです。
type Concurrent struct {
	extractor      ports.Extractor
	maxConcurrency int
	rateLimit      time.Duration
	limiter        *rate.Limiter
}

// New は Concurrent 構造体を初期化します。
func New(extractor ports.Extractor, opts ...Option) *Concurrent {
	c := &Concurrent{
		extractor:      extractor,
		maxConcurrency: DefaultMaxConcurrency,
		rateLimit:      DefaultRateLimit,
	}

	for _, opt := range opts {
		opt(c)
	}

	c.limiter = rate.NewLimiter(rate.Every(c.rateLimit), 1)
	return c
}

// Run は複数の URL に対して並列スクレイピングを実行します。
func (c *Concurrent) Run(ctx context.Context, urls []string) []ports.URLResult {
	g, gCtx := errgroup.WithContext(ctx)
	g.SetLimit(c.maxConcurrency)

	resultsChan := make(chan ports.URLResult, len(urls))

	for _, url := range urls {
		g.Go(func() error {
			if err := c.limiter.Wait(gCtx); err != nil {
				resultsChan <- ports.URLResult{URL: url, Error: err}
				return nil
			}

			content, hasBodyFound, err := c.extractor.FetchAndExtractText(gCtx, url)

			var extractErr error
			if err != nil {
				extractErr = fmt.Errorf("抽出失敗: %w", err)
			} else if !hasBodyFound {
				extractErr = fmt.Errorf("URL %s から本文を抽出できませんでした", url)
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
