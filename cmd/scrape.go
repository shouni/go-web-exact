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

	// 1. Scraperの初期化 (記憶された NewParallelScraper を利用)
	scraper := scraper.NewParallelScraper(extractor, concurrency)

	// 2. タイムアウト設定は、個々のリクエストではなく、全体の処理に適用します。
	// extractCmdと統一するため、クライアントタイムアウト (Flags.TimeoutSec) の2倍を全体のタイムアウトとします。
	overallTimeout := time.Duration(Flags.TimeoutSec) * 2 * time.Second
	if Flags.TimeoutSec == 0 {
		overallTimeout = DefaultOverallTimeoutIfClientTimeoutIsZero
	}

	// 3. 全体処理のコンテキストを設定
	ctx, cancel := context.WithTimeout(context.Background(), overallTimeout)
	defer cancel()

	log.Printf("並列スクレイピング開始 (対象URL数: %d, 最大同時実行数: %d, 全体タイムアウト: %s)\n",
		len(urls), scraper.DefaultMaxConcurrency, overallTimeout)

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

var scrapeCmd = &cobra.Command{
	Use:   "scrape",
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

		// 2. 処理対象URLのリストを決定
		var urls []string

		if inputURLs != "" {
			// --urls フラグからURLリストを取得
			urls = strings.Split(inputURLs, ",")
		} else {
			// 標準入力からURLを一行ずつ読み込む
			log.Println("URLが指定されていないため、標準入力からURLを読み込みます (Ctrl+DまたはEOFで終了)...")
			scanner := bufio.NewScanner(os.Stdin)
			for scanner.Scan() {
				url := strings.TrimSpace(scanner.Text())
				if url != "" {
					urls = append(urls, url)
				}
			}
			if err := scanner.Err(); err != nil {
				return fmt.Errorf("標準入力の読み取りエラー: %w", err)
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
	scrapeCmd.Flags().StringVarP(&inputURLs, "urls", "u", "",
		"抽出対象のカンマ区切りURLリスト (例: url1,url2,url3)")

	// --concurrency フラグ: 並列実行数の指定
	scrapeCmd.Flags().IntVarP(&concurrency, "concurrency", "c",
		scraper.DefaultMaxConcurrency,
		fmt.Sprintf("最大並列実行数 (デフォルト: %d)", scraper.DefaultMaxConcurrency))

	// NOTE: --urls フラグは必須ではありません。標準入力からの入力も許可します。
}
