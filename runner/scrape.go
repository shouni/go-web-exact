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
	// DefaultInitialDelay は並列実行後のデフォルト待機時間です。
	DefaultInitialDelay = 5 * time.Second
	// DefaultRetryDelay はリトライ開始前のデフォルト待機時間です。
	DefaultRetryDelay = 3 * time.Second
	// PhaseContent はこの実行フェーズの識別子です。
	PhaseContent = "ContentExtraction"
)

// ScrapeRunner は、並列スクレイピングと失敗時の逐次リトライを制御する指揮官です。
type ScrapeRunner struct {
	scraper            ports.Scraper
	extractor          ports.Extractor
	initialScrapeDelay time.Duration
	retryScrapeDelay   time.Duration
}

// Option は ScrapeRunner の挙動をカスタマイズするための関数型です。
type Option func(*ScrapeRunner)

// WithInitialDelay は初回実行後の待機時間を設定します。
func WithInitialDelay(d time.Duration) Option {
	return func(r *ScrapeRunner) { r.initialScrapeDelay = d }
}

// WithRetryDelay はリトライ前の待機時間を設定します。
func WithRetryDelay(d time.Duration) Option {
	return func(r *ScrapeRunner) { r.retryScrapeDelay = d }
}

// NewScrapeRunner は依存関係とオプションを適用して Runner を生成します。
func NewScrapeRunner(scraper ports.Scraper, extractor ports.Extractor, opts ...Option) *ScrapeRunner {
	r := &ScrapeRunner{
		scraper:            scraper,
		extractor:          extractor,
		initialScrapeDelay: DefaultInitialDelay,
		retryScrapeDelay:   DefaultRetryDelay,
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// Run は、URLリストに対して一括抽出と自動リトライのパイプラインを実行します。
func (r *ScrapeRunner) Run(ctx context.Context, urls []string) []ports.URLResult {
	slog.Info("Phase: "+PhaseContent+" - Start", slog.Int("count", len(urls)))

	// 1. 初回並列実行
	results := r.scraper.Run(ctx, urls)

	// 2. 負荷軽減のための待機 (Context キャンセルを考慮)
	if err := r.wait(ctx, r.initialScrapeDelay); err != nil {
		slog.Warn("待機中にコンテキストが終了しました。取得済みの結果のみ返却します。", slog.Any("error", err))
		successes, _ := splitResults(results)
		return successes
	}

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
		slog.Error("有効なコンテンツを一件も取得できませんでした。URLまたは通信状況を確認してください。")
		return []ports.URLResult{}
	}

	slog.Info("Phase: "+PhaseContent+" - Completed",
		slog.Int("total", len(urls)),
		slog.Int("success", len(successes)),
		slog.Int("initial", initialCount),
		slog.Int("retry_gain", len(successes)-initialCount),
	)

	return successes
}

// retry は、失敗したURLに対して逐次抽出を試みます。
func (r *ScrapeRunner) retry(ctx context.Context, urls []string) []ports.URLResult {
	slog.Warn("抽出失敗URLのリトライ準備中...",
		slog.Int("count", len(urls)),
		slog.Duration("delay", r.retryScrapeDelay))

	// リトライ前の待機 (Context キャンセルを考慮)
	if err := r.wait(ctx, r.retryScrapeDelay); err != nil {
		slog.Warn("リトライ待機中にコンテキストが終了しました。")
		return []ports.URLResult{}
	}

	var results []ports.URLResult
	for _, url := range urls {
		// ループ内でもキャンセルをチェックするのだ
		select {
		case <-ctx.Done():
			slog.Warn("リトライ処理中にコンテキストが終了しました。", slog.String("url", url))
			return results
		default:
			slog.Info("逐次リトライ中", slog.String("url", url))

			content, hasBody, err := r.extractor.FetchAndExtractText(ctx, url)

			var extractErr error
			if err != nil {
				extractErr = fmt.Errorf("リトライ抽出失敗: %w", err)
			} else if content == "" || !hasBody {
				extractErr = fmt.Errorf("URL %s から有効な本文を検出できませんでした", url)
			}

			if extractErr != nil {
				slog.Error("リトライ最終失敗",
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
	}
	return results
}

// wait は time.After と Context.Done を監視して、安全に待機するヘルパーです。
func (r *ScrapeRunner) wait(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
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

// simplifyError は、ログ出力用に冗長なエラーメッセージを整理します。
// TODO: 下位パッケージでカスタムエラー型を定義し、errors.As による判定へ移行することを推奨。
func simplifyError(err error) string {
	msg := err.Error()
	// 暫定的な文字列パース（将来の型判定導入までの繋ぎなのだ）
	if idx := strings.Index(msg, ", ボディ: <!"); idx != -1 {
		msg = msg[:idx]
	}
	if idx := strings.LastIndex(msg, "最終エラー:"); idx != -1 {
		return strings.TrimSpace(msg[idx:])
	}
	return msg
}
