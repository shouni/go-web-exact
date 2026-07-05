package scraper

import "time"

// Option はConcurrentの設定を行うための関数型です。
type Option func(*Concurrent)

// WithMaxConcurrency は最大並列を設定します。
func WithMaxConcurrency(maxConcurrency int) Option {
	return func(c *Concurrent) {
		if maxConcurrency > 0 {
			c.maxConcurrency = maxConcurrency
		}
	}
}

// WithRateLimit はリクエスト間のレート制限間隔を設定します。
func WithRateLimit(d time.Duration) Option {
	return func(c *Concurrent) {
		if d > 0 {
			c.rateLimit = d
		}
	}
}
