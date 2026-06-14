package runner

import (
	"context"
	"errors"
	"io"
	"strings"
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
	extractFunc       func(ctx context.Context, url string) (string, bool, error)
	extractReaderFunc func(ctx context.Context, reader io.Reader) (string, bool, error)
}

func (m *mockExtractor) FetchAndExtractText(ctx context.Context, url string) (string, bool, error) {
	return m.extractFunc(ctx, url)
}

func (m *mockExtractor) ExtractText(ctx context.Context, reader io.Reader) (string, bool, error) {
	if m.extractReaderFunc != nil {
		return m.extractReaderFunc(ctx, reader)
	}
	return "", false, errors.New("unexpected ExtractText call")
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

	t.Run("初回結果がHTMLの場合は本文を解析する", func(t *testing.T) {
		scraper := &mockScraper{
			runFunc: func(ctx context.Context, urls []string) []ports.URLResult {
				return []ports.URLResult{
					{
						URL:         "http://html.com",
						Content:     "<html><body><main><p>HTML body text long enough to extract.</p></main></body></html>",
						ContentType: "text/html; charset=utf-8",
					},
				}
			},
		}
		extractor := &mockExtractor{
			extractReaderFunc: func(ctx context.Context, reader io.Reader) (string, bool, error) {
				body, err := io.ReadAll(reader)
				if err != nil {
					return "", false, err
				}
				if !strings.Contains(string(body), "<html>") {
					return "", false, errors.New("HTMLが渡されていません")
				}
				return "extracted body", true, nil
			},
		}

		r := NewScrapeRunner(scraper, extractor, fastOpts...)
		results := r.Run(context.Background(), []string{"http://html.com"})

		if len(results) != 1 {
			t.Fatalf("結果は1件であるべきだが %d 件だったのだ", len(results))
		}
		if results[0].Content != "extracted body" {
			t.Errorf("HTML解析後の本文が返るべきなのだ。got: %q", results[0].Content)
		}
	})

	t.Run("初回結果がHTML以外の場合は解析しない", func(t *testing.T) {
		scraper := &mockScraper{
			runFunc: func(ctx context.Context, urls []string) []ports.URLResult {
				return []ports.URLResult{
					{URL: "http://plain.com", Content: "plain extracted body", ContentType: "text/plain"},
				}
			},
		}

		r := NewScrapeRunner(scraper, &mockExtractor{}, fastOpts...)
		results := r.Run(context.Background(), []string{"http://plain.com"})

		if len(results) != 1 {
			t.Fatalf("結果は1件であるべきだが %d 件だったのだ", len(results))
		}
		if results[0].Content != "plain extracted body" {
			t.Errorf("HTML以外はそのまま返るべきなのだ。got: %q", results[0].Content)
		}
	})

	t.Run("HTML解析は入力スライスを変更しない", func(t *testing.T) {
		original := []ports.URLResult{
			{
				URL:         "http://html.com",
				Content:     "<html><body><main><p>HTML body text long enough to extract.</p></main></body></html>",
				ContentType: "text/html",
			},
		}
		extractor := &mockExtractor{
			extractReaderFunc: func(ctx context.Context, reader io.Reader) (string, bool, error) {
				return "extracted body", true, nil
			},
		}

		r := NewScrapeRunner(&mockScraper{}, extractor, fastOpts...)
		results := r.extractHTMLResults(context.Background(), original)

		if original[0].Content == "extracted body" {
			t.Fatal("入力スライスは変更されないべきなのだ")
		}
		if results[0].Content != "extracted body" {
			t.Errorf("返却スライスには解析後の本文が入るべきなのだ。got: %q", results[0].Content)
		}
	})

	t.Run("キャンセル済みの場合はHTML解析を開始しない", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		original := []ports.URLResult{
			{
				URL:         "http://html.com",
				Content:     "<html><body><main><p>HTML body text long enough to extract.</p></main></body></html>",
				ContentType: "text/html",
			},
		}
		extractor := &mockExtractor{
			extractReaderFunc: func(ctx context.Context, reader io.Reader) (string, bool, error) {
				t.Fatal("キャンセル済みの場合はExtractTextを呼ばないべきなのだ")
				return "", false, nil
			},
		}

		r := NewScrapeRunner(&mockScraper{}, extractor, fastOpts...)
		results := r.extractHTMLResults(ctx, original)

		if results[0].Error == nil {
			t.Fatal("キャンセル済みの場合はエラーが設定されるべきなのだ")
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
