package builder

import (
	"context"
	"errors"
	"testing"

	"github.com/shouni/go-web-exact/v2/runner"
	"github.com/shouni/go-web-exact/v2/scraper"
)

// mockFetcher は ports.Fetcher のモックなのだ
type mockFetcher struct {
	fetchFunc func(ctx context.Context, url string) ([]byte, error)
}

func (m *mockFetcher) FetchBytes(ctx context.Context, url string) ([]byte, error) {
	return m.fetchFunc(ctx, url)
}

func TestNew(t *testing.T) {
	t.Run("Fetcherがnilの場合はエラーを返す", func(t *testing.T) {
		b, err := New(nil, nil)
		if err == nil {
			t.Fatal("nil Fetcherの場合はエラーが返るべきなのだ")
		}
		if b != nil {
			t.Error("エラー時はBuilderがnilであるべきなのだ")
		}
	})

	t.Run("有効なFetcherの場合はBuilderとScrapeRunnerを構築する", func(t *testing.T) {
		fetcher := &mockFetcher{
			fetchFunc: func(_ context.Context, _ string) ([]byte, error) {
				return []byte("<html><body><main><p>Body text long enough to extract.</p></main></body></html>"), nil
			},
		}

		b, err := New(fetcher, nil)
		if err != nil {
			t.Fatalf("エラーは発生しないはずなのだ: %v", err)
		}
		if b == nil {
			t.Fatal("Builderはnilであってはならないのだ")
		}
		if b.ScrapeRunner() == nil {
			t.Fatal("ScrapeRunnerはnilであってはならないのだ")
		}
	})

	t.Run("scraper.Optionが正しく反映される", func(t *testing.T) {
		fetcher := &mockFetcher{
			fetchFunc: func(_ context.Context, _ string) ([]byte, error) {
				return nil, errors.New("呼ばれないはずなのだ")
			},
		}

		opts := []scraper.Option{scraper.WithMaxConcurrency(1)}
		b, err := New(fetcher, opts)
		if err != nil {
			t.Fatalf("エラーは発生しないはずなのだ: %v", err)
		}
		if b.ScrapeRunner() == nil {
			t.Fatal("ScrapeRunnerはnilであってはならないのだ")
		}
	})

	t.Run("ScrapeRunnerはrunner.ScrapeRunnerとして構築される", func(t *testing.T) {
		fetcher := &mockFetcher{
			fetchFunc: func(_ context.Context, _ string) ([]byte, error) {
				return []byte("<html><body><main><p>Body text long enough to extract.</p></main></body></html>"), nil
			},
		}

		b, err := New(fetcher, nil)
		if err != nil {
			t.Fatalf("エラーは発生しないはずなのだ: %v", err)
		}

		r := b.ScrapeRunner()
		if _, ok := r.(*runner.ScrapeRunner); !ok {
			t.Fatalf("ScrapeRunnerの実装型が想定と異なるのだ: %T", r)
		}
	})
}
