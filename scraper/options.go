package scraper

import "time"

// Option はParallelScraperの設定を行うための関数型です。
type Option func(*ParallelScraper)

// WithMaxConcurrency は最大並列を設定します。
func WithMaxConcurrency(max int) Option {
	return func(c *ParallelScraper) {
		if max > 0 {
			c.maxConcurrency = max
		}
	}
}

// WithRateLimit はリトライの初期間隔を設定します。
func WithRateLimit(d time.Duration) Option {
	return func(c *ParallelScraper) {
		if d > 0 {
			c.rateLimit = d
		}
	}
}
