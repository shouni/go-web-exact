package runner

import (
	"context"
	"errors"
	"testing"

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
	ctx := context.Background()

	t.Run("初回で全件成功する場合", func(t *testing.T) {
		scraper := &mockScraper{
			runFunc: func(ctx context.Context, urls []string) []ports.URLResult {
				return []ports.URLResult{
					{URL: "http://ok1.com", Content: "body1"},
					{URL: "http://ok2.com", Content: "body2"},
				}
			},
		}
		extractor := &mockExtractor{} // リトライは走らないので未定義でOK

		r := NewScrapeRunner(scraper, extractor)
		results := r.Run(ctx, []string{"http://ok1.com", "http://ok2.com"})

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

		r := NewScrapeRunner(scraper, extractor)
		results := r.Run(ctx, []string{"http://ok.com", "http://retry.com"})

		if len(results) != 2 {
			t.Errorf("リトライを含めて2件成功すべきなのだ。got: %d", len(results))
		}

		// 内容の確認
		foundRetry := false
		for _, res := range results {
			if res.URL == "http://retry.com" && res.Content == "body_retried" {
				foundRetry = true
			}
		}
		if !foundRetry {
			t.Error("リトライで取得したコンテンツが含まれていないのだ")
		}
	})

	t.Run("リトライしても失敗する場合", func(t *testing.T) {
		scraper := &mockScraper{
			runFunc: func(ctx context.Context, urls []string) []ports.URLResult {
				return []ports.URLResult{
					{URL: "http://ok.com", Content: "body_ok"},
					{URL: "http://fail.com", Error: errors.New("hard error")},
				}
			},
		}
		extractor := &mockExtractor{
			extractFunc: func(ctx context.Context, url string) (string, bool, error) {
				return "", false, errors.New("still failing")
			},
		}

		r := NewScrapeRunner(scraper, extractor)
		results := r.Run(ctx, []string{"http://ok.com", "http://fail.com"})

		// 成功した1件だけが返るはずなのだ
		if len(results) != 1 {
			t.Errorf("成功した1件だけが返るべきなのだ。got: %d", len(results))
		}
		if results[0].URL != "http://ok.com" {
			t.Errorf("成功したURLが違うのだ。got: %s", results[0].URL)
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

		r := NewScrapeRunner(scraper, extractor)
		results := r.Run(ctx, []string{"http://fail.com"})

		if results == nil || len(results) != 0 {
			t.Error("全件失敗時は空のスライスが返るべきなのだ")
		}
	})
}
