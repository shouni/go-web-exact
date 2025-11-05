package cmd

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/shouni/go-web-exact/v2/pkg/extract"
	"github.com/shouni/go-web-exact/v2/pkg/scraper"
	"github.com/spf13/cobra"
)

// コマンドラインフラグ変数を定義
var (
	inputURLs   string // --urls フラグで受け取るカンマ区切りのURLリスト
	concurrency int    // --concurrency フラグで受け取る並列実行数
)

// runScrapePipeline は、並列スクレイピングを実行するメインロジックです。
func runScrapePipeline(urls []string, extractor *extract.Extractor, concurrency int) {

	// 1. Scraperの初期化 (NewParallelScraper を利用)
	scraper := scraper.NewParallelScraper(extractor, concurrency)

	// 2. タイムアウト設定: (修正点1に対応)
	// クライアントタイムアウト(Flags.TimeoutSec)を基に全体のタイムアウトを計算し、一貫性を保つ。
	var clientTimeout time.Duration
	if Flags.TimeoutSec == 0 {
		clientTimeout = defaultTimeoutSec * time.Second
	} else {
		clientTimeout = time.Duration(Flags.TimeoutSec) * time.Second
	}
	// extractorCmdと同様に、全体のタイムアウトをクライアントタイムアウトの2倍とする
	overallTimeout := clientTimeout * 2

	// 3. 全体処理のコンテキストを設定
	ctx, cancel := context.WithTimeout(context.Background(), overallTimeout)
	defer cancel()

	log.Printf("並列スクレイピング開始 (対象URL数: %d, 最大同時実行数: %d, 全体タイムアウト: %s)\n",
		len(urls), concurrency, overallTimeout)

	// 4. メインロジックの実行
	results := scraper.ScrapeInParallel(ctx, urls)

	// 5. 結果の出力
	fmt.Println("--- 並列スクレイピング結果 ---")

	successCount := 0
	errorCount := 0

	for i, res := range results {
		if res.Error != nil {
			errorCount++
			fmt.Printf("❌ [%d] %s\n", i+1, res.URL)
			fmt.Printf("     エラー: %v\n", res.Error)
		} else {
			successCount++
			fmt.Printf("✅ [%d] %s\n", i+1, res.URL)
			fmt.Printf("     抽出コンテンツの長さ: %d 文字\n", len(res.Content))

			// デバッグ用にコンテンツのプレビューを表示
			if len(res.Content) > 100 {
				fmt.Printf("     プレビュー: %s...\n", res.Content[:100])
			} else {
				fmt.Printf("     コンテンツ: %s\n", res.Content)
			}
		}
	}

	fmt.Println("-------------------------------")
	fmt.Printf("完了: 成功 %d 件, 失敗 %d 件\n", successCount, errorCount)
}

// scrapeCmd から scraperCmd に名称変更
var scraperCmd = &cobra.Command{
	Use:   "scraper",
	Short: "複数のURLを並列で処理し、コンテンツを抽出します",
	Long:  `--urls フラグでカンマ区切りのURLリストを受け取るか、標準入力からURLを一行ずつ読み込み、指定された最大同時実行数で並列抽出を実行します。`,
	Args:  cobra.NoArgs, // 位置引数は取らない

	RunE: func(cmd *cobra.Command, args []string) error {

		// 1. 依存性の初期化 (Fetcher -> Extractor)
		fetcher := GetGlobalFetcher()
		if fetcher == nil {
			return fmt.Errorf("HTTPクライアントの取得に失敗しました")
		}
		extractor, err := extract.NewExtractor(fetcher)
		if err != nil {
			return fmt.Errorf("Extractorの初期化エラー: %w", err)
		}

		// 2. 処理対象URLのリストを決定 (修正点2に対応: ensureSchemeを適用)
		var urls []string
		var rawURLs []string

		// 2-1. フラグからの読み込み
		if inputURLs != "" {
			rawURLs = strings.Split(inputURLs, ",")
		} else {
			// 2-2. 標準入力からの読み込み
			log.Println("URLが指定されていないため、標準入力からURLを読み込みます (Ctrl+DまたはEOFで終了)...")
			scanner := bufio.NewScanner(os.Stdin)
			for scanner.Scan() {
				rawURLs = append(rawURLs, scanner.Text())
			}
			if err := scanner.Err(); err != nil {
				return fmt.Errorf("標準入力の読み取りエラー: %w", err)
			}
		}

		// 2-3. URLスキーム補完とバリデーションの適用
		for _, u := range rawURLs {
			u = strings.TrimSpace(u)
			if u != "" {
				// 💡 ensureScheme を呼び出す
				processed, err := ensureScheme(u)
				if err != nil {
					return fmt.Errorf("URLスキームの処理エラー (%s): %w", u, err)
				}
				urls = append(urls, processed)
			}
		}

		if len(urls) == 0 {
			return fmt.Errorf("処理対象のURLが一つも指定されていません")
		}

		// 3. メインロジックの実行
		runScrapePipeline(urls, extractor, concurrency)

		return nil
	},
}

func init() {
	// --urls フラグ: カンマ区切りのURLリスト
	scraperCmd.Flags().StringVarP(&inputURLs, "urls", "u", "",
		"抽出対象のカンマ区切りURLリスト (例: url1,url2,url3)")

	// --concurrency フラグ: 並列実行数の指定
	scraperCmd.Flags().IntVarP(&concurrency, "concurrency", "c",
		scraper.DefaultMaxConcurrency,
		fmt.Sprintf("最大並列実行数 (デフォルト: %d)", scraper.DefaultMaxConcurrency))
}
