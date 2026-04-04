package runner

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/shouni/go-web-exact/v2/ports"
)

// mockScraper は ports.Scraper のモックなのだ
type mockScraper struct {
	runFunc func(ctx context.Context, urls []string) []ports.URLResult
}

func (m *mockScraper) Run(ctx context.Context, urls []string) []ports.URLResult {
	return m.runFunc(ctx, urls)
}

// mockExtractor は ports.Extractor のモックなのだ
type mockExtractor struct {
	extractFunc func(ctx context.Context, url string) (string, bool, error)
}

func (m *mockExtractor) FetchAndExtractText(ctx context.Context, url string) (string, bool, error) {
	return m.extractFunc(ctx, url)
}

func TestScrapeRunner_Run(t *testing.T) {
	// 共通設定: テストを高速化するためにディレイを最小にするのだ！
	fastOpts := []Option{
		WithInitialDelay(1 * time.Millisecond),
		WithRetryDelay(1 * time.Millisecond),
	}

	t.Run("初回で全件成功する場合", func(t *testing.T) {
		scraper := &mockScraper{
			runFunc: func(ctx context.Context, urls []string) []ports.URLResult {
				return []ports.URLResult{
					{URL: "http://ok1.com", Content: "body1"},
					{URL: "http://ok2.com", Content: "body2"},
				}
			},
		}
		r := NewScrapeRunner(scraper, &mockExtractor{}, fastOpts...)
		results := r.Run(context.Background(), []string{"http://ok1.com", "http://ok2.com"})

		if len(results) != 2 {
			t.Errorf("結果は2件であるべきだが %d 件だったのだ", len(results))
		}
	})

	t.Run("初回で一部失敗しリトライで救済される場合", func(t *testing.T) {
		scraper := &mockScraper{
			runFunc: func(ctx context.Context, urls []string) []ports.URLResult {
				return []ports.URLResult{
					{URL: "http://ok.com", Content: "body_ok"},
					{URL: "http://retry.com", Error: errors.New("temporary error")},
				}
			},
		}
		extractor := &mockExtractor{
			extractFunc: func(ctx context.Context, url string) (string, bool, error) {
				if url == "http://retry.com" {
					return "body_retried", true, nil
				}
				return "", false, errors.New("unexpected call")
			},
		}

		r := NewScrapeRunner(scraper, extractor, fastOpts...)
		results := r.Run(context.Background(), []string{"http://ok.com", "http://retry.com"})

		if len(results) != 2 {
			t.Errorf("リトライを含めて2件成功すべきなのだ。got: %d", len(results))
		}
	})

	t.Run("リトライ中にコンテキストがキャンセルされた場合", func(t *testing.T) {
		scraper := &mockScraper{
			runFunc: func(ctx context.Context, urls []string) []ports.URLResult {
				return []ports.URLResult{
					{URL: "http://ok.com", Content: "body_ok"},
					{URL: "http://retry.com", Error: errors.New("fail")},
				}
			},
		}
		// 意図的にキャンセル済みのコンテキストを作成するのだ
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		r := NewScrapeRunner(scraper, &mockExtractor{}, fastOpts...)
		results := r.Run(ctx, []string{"http://ok.com", "http://retry.com"})

		// キャンセルされた場合、リトライは実行されず、最初の成功分だけが返るはずなのだ
		if len(results) != 1 {
			t.Errorf("キャンセル時はリトライが走らず1件のみ返るべきなのだ。got: %d", len(results))
		}
	})

	t.Run("全ての取得に失敗し空のスライスが返る場合", func(t *testing.T) {
		scraper := &mockScraper{
			runFunc: func(ctx context.Context, urls []string) []ports.URLResult {
				return []ports.URLResult{{URL: "http://fail.com", Error: errors.New("fail")}}
			},
		}
		extractor := &mockExtractor{
			extractFunc: func(ctx context.Context, url string) (string, bool, error) {
				return "", false, errors.New("fail again")
			},
		}

		r := NewScrapeRunner(scraper, extractor, fastOpts...)
		results := r.Run(context.Background(), []string{"http://fail.com"})

		if len(results) != 0 {
			t.Error("全件失敗時は空のスライスが返るべきなのだ")
		}
	})
}
