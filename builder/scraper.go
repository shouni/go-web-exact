// Package builder は、Fetcher から Extractor・Scraper・ScrapeRunner までの依存関係を組み立てます。
package builder

import (
	"fmt"

	"github.com/shouni/go-web-exact/v2/extract"
	"github.com/shouni/go-web-exact/v2/ports"
	"github.com/shouni/go-web-exact/v2/runner"
	"github.com/shouni/go-web-exact/v2/scraper"
)

// Builder は依存関係を管理し、適切なRunnerを生成します。
type Builder struct {
	fetcher ports.Fetcher
	runner  ports.ScrapeRunner
}

// New は、ScraperBuilderのインスタンスを返します。
// runnerOpts で ScrapeRunner の挙動（HTMLワーカー数、待機時間など）をカスタマイズできます。
func New(fetcher ports.Fetcher, opts []scraper.Option, runnerOpts ...runner.Option) (*Builder, error) {
	extractor, err := extract.NewExtractor(fetcher)
	if err != nil {
		return nil, fmt.Errorf("extractorの初期化エラー: %w", err)
	}
	coreScraper := scraper.New(extractor, opts...)
	scrapeRunner := runner.NewScrapeRunner(coreScraper, extractor, runnerOpts...)

	return &Builder{
		fetcher: fetcher,
		runner:  scrapeRunner,
	}, nil
}

// ScrapeRunner は、構築に利用される ScrapeRunner を返します。
func (s *Builder) ScrapeRunner() ports.ScrapeRunner {
	return s.runner
}
