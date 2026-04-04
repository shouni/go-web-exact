package runner

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/shouni/go-web-exact/v2/ports"
)

const (
	InitialScrapeDelay = 5 * time.Second
	RetryScrapeDelay   = 3 * time.Second
	PhaseContent       = "ContentExtraction"
)

// ScrapeRunner は、並列スクレイピングと逐次リトライのパイプラインを制御します。
type ScrapeRunner struct {
	scraper   ports.Scraper
	extractor ports.Extractor
}

// NewScrapeRunner は ScrapeRunner の新しいインスタンスを作成します。
func NewScrapeRunner(scraper ports.Scraper, extractor ports.Extractor) *ScrapeRunner {
	return &ScrapeRunner{
		scraper:   scraper,
		extractor: extractor,
	}
}

// Run は、URLリストに対して一括抽出を試み、失敗したURLを自動でリトライします。
func (r *ScrapeRunner) Run(ctx context.Context, urls []string) []ports.URLResult {
	slog.Info("フェーズ1 - Webコンテンツの並列抽出を開始します。", slog.Int("count", len(urls)))

	// 1. 初回並列実行
	results := r.scraper.Run(ctx, urls)

	// 2. 負荷軽減のための待機
	slog.Info("並列抽出が完了しました。待機後、結果を評価します。",
		slog.String("phase", PhaseContent),
		slog.Duration("delay", InitialScrapeDelay))
	time.Sleep(InitialScrapeDelay)

	// 3. 結果の分類
	successes, failedURLs := splitResults(results)
	initialCount := len(successes)

	// 4. 失敗したURLに対するリトライ処理
	if len(failedURLs) > 0 {
		retriedSuccesses := r.retry(ctx, failedURLs)
		successes = append(successes, retriedSuccesses...)
	}

	// 5. 最終評価
	if len(successes) == 0 {
		slog.Error("処理可能なコンテンツを一件も取得できませんでした。")
		return []ports.URLResult{}
	}

	slog.Info("コンテンツ取得完了",
		slog.Int("total", len(urls)),
		slog.Int("success", len(successes)),
		slog.Int("initial", initialCount),
		slog.Int("retry_gain", len(successes)-initialCount),
	)

	return successes
}

// retry は、失敗したURLに対して逐次抽出を試みます。
func (r *ScrapeRunner) retry(ctx context.Context, urls []string) []ports.URLResult {
	slog.Warn("抽出失敗URLのリトライを開始します。",
		slog.Int("count", len(urls)),
		slog.Duration("delay", RetryScrapeDelay))

	time.Sleep(RetryScrapeDelay)

	var results []ports.URLResult
	for _, url := range urls {
		slog.Info("リトライ中", slog.String("url", url))

		content, hasBody, err := r.extractor.FetchAndExtractText(ctx, url)

		var extractErr error
		if err != nil {
			extractErr = fmt.Errorf("リトライ抽出失敗: %w", err)
		} else if content == "" || !hasBody {
			extractErr = fmt.Errorf("URL %s から本文を検出できませんでした", url)
		}

		if extractErr != nil {
			slog.Error("リトライ失敗",
				slog.String("url", url),
				slog.String("error", simplifyError(extractErr)))
			continue
		}

		slog.Info("リトライ成功", slog.String("url", url))
		results = append(results, ports.URLResult{
			URL:     url,
			Content: content,
		})
	}
	return results
}

// splitResults は結果を成功と失敗したURLリストに分離します。
func splitResults(results []ports.URLResult) (successes []ports.URLResult, failed []string) {
	for _, res := range results {
		if res.Error != nil || res.Content == "" {
			failed = append(failed, res.URL)
		} else {
			successes = append(successes, res)
		}
	}
	return successes, failed
}

// simplifyError は、ログ出力用にエラーメッセージを整形します。
func simplifyError(err error) string {
	msg := err.Error()
	if idx := strings.Index(msg, ", ボディ: <!"); idx != -1 {
		msg = msg[:idx]
	}
	if idx := strings.LastIndex(msg, "最終エラー:"); idx != -1 {
		return strings.TrimSpace(msg[idx:])
	}
	return msg
}
