// Package scraper は、レート制限付きの並列フェッチエンジンを提供します。
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

// Concurrent は、並列かつレート制限を考慮してURLを取得するエンジンです。
// HTML解析は行わず、取得した生データと Content-Type をそのまま返します（解析は runner が担当）。
type Concurrent struct {
	fetcher        ports.Fetcher
	maxConcurrency int
	rateLimit      time.Duration
	limiter        *rate.Limiter
}

// New は Concurrent 構造体を初期化します。
func New(fetcher ports.Fetcher, opts ...Option) *Concurrent {
	c := &Concurrent{
		fetcher:        fetcher,
		maxConcurrency: DefaultMaxConcurrency,
		rateLimit:      DefaultRateLimit,
	}

	for _, opt := range opts {
		opt(c)
	}

	c.limiter = rate.NewLimiter(rate.Every(c.rateLimit), 1)
	return c
}

// Run は複数の URL に対して並列フェッチを実行します。
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

			body, contentType, err := c.fetcher.FetchBytes(gCtx, url)
			if err != nil {
				resultsChan <- ports.URLResult{URL: url, Error: fmt.Errorf("取得失敗: %w", err)}
				return nil
			}

			resultsChan <- ports.URLResult{URL: url, Content: string(body), ContentType: contentType}
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
