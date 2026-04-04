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
func New(fetcher ports.Fetcher, opts []scraper.Option) (*Builder, error) {
	extractor, err := extract.NewExtractor(fetcher)
	if err != nil {
		return nil, fmt.Errorf("Extractorの初期化エラー: %w", err)
	}
	coreScraper := scraper.New(extractor, opts...)
	scrapeRunner := runner.NewScrapeRunner(coreScraper, extractor)

	return &Builder{
		fetcher: fetcher,
		runner:  scrapeRunner,
	}, nil
}

// ScrapeRunner は、構築に利用される ScrapeRunner を返します。
func (s *Builder) ScrapeRunner() ports.ScrapeRunner {
	return s.runner
}
