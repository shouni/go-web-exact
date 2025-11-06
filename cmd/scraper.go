package cmd

import (
	"fmt"

	"github.com/shouni/go-http-kit/pkg/httpkit"
	"github.com/shouni/go-web-exact/v2/pkg/extract"
	"github.com/shouni/go-web-exact/v2/pkg/scraper"
	"github.com/spf13/cobra"
)

// コマンドラインフラグ変数を定義
var (
	concurrency int // --concurrency フラグで受け取る並列実行数
)

// runScrapePipeline は、並列スクレイピングを実行するメインロジックです。
func runScrapePipeline(urls []string, extractor *extract.Extractor, concurrency int) {

	/**
	// 1. Scraperの初期化 (NewParallelScraper を利用)
	parallelScraper := scraper.NewParallelScraper(extractor, concurrency)

	// 2. タイムアウト設定:
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

	// 4. メインロジックの実行: scraper.ScrapeInParallel が内部で extractor.FetchAndExtractText を呼び出します
	results := parallelScraper.parallelScraper(ctx, urls)

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
	*/
}

// scraperCmd は、フィードから抽出したURLを並列で処理し、コンテンツを抽出します。
var scraperCmd = &cobra.Command{
	Use:   "scraper",
	Short: "RSSフィードから抽出した複数のURLを並列で処理し、コンテンツを抽出します",
	Long:  `--url フラグで指定されたRSS/Atomフィードを解析し、含まれる記事のURLを抽出し、指定された最大同時実行数で並列抽出を実行します。`,
	Args:  cobra.NoArgs, // 位置引数は取らない

	RunE: func(cmd *cobra.Command, args []string) error {

		// 1. 依存性の初期化 (Fetcher -> Extractor)
		fetcher := httpkit.New(defaultMaxRetries)
		if fetcher == nil {
			return fmt.Errorf("HTTPクライアントの取得に失敗しました")
		}
		//// httpkit.Client が feed.Fetcher インターフェースを満たすことを前提とする
		//parser, err := feed.NewParser(fetcher)
		//if err != nil {
		//	return fmt.Errorf("Parserの初期化エラー: %w", err)
		//}
		//// httpkit.Client が extract.Fetcher インターフェースを満たすことを前提とする
		//extractor, err := extract.NewExtractor(fetcher)
		//if err != nil {
		//	return fmt.Errorf("Extractorの初期化エラー: %w", err)
		//}
		//
		//// 2. フィード取得のための短時間のコンテキストを設定
		//ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		//defer cancel()
		//
		//// 3. フィードの取得とパースを実行
		//log.Printf("フィードURLを解析中: %s\n", feedURL)
		//rssFeed, err := parser.FetchAndParse(ctx, feedURL)
		//if err != nil {
		//	return fmt.Errorf("フィードの処理エラー: %w", err)
		//}
		//
		//// 4. RSSフィードから記事のURLを抽出
		//var urls []string
		//for _, item := range rssFeed.Items {
		//	if item.Link != "" {
		//		urls = append(urls, item.Link)
		//	}
		//}
		//
		//log.Printf("フィードから %d 件のURLを抽出しました。\n", len(urls))
		//
		//if len(urls) == 0 {
		//	return fmt.Errorf("フィード (%s) から処理対象のURLが一つも抽出されませんでした", feedURL)
		//}
		//
		//// 5. メインロジックの実行
		//// runScrapePipeline は並列処理のコンテキストを内部で設定します。
		//runScrapePipeline(urls, extractor, concurrency)

		return nil
	},
}

func init() {
	// --url フラグ: 解析対象のフィードURL (RSS/Atom)
	scraperCmd.Flags().StringVarP(&feedURL, "url", "u", "https://news.yahoo.co.jp/rss/categories/it.xml", "解析対象のフィードURL (RSS/Atom)")

	// --concurrency フラグ: 並列実行数の指定
	scraperCmd.Flags().IntVarP(&concurrency, "concurrency", "c",
		scraper.DefaultMaxConcurrency,
		fmt.Sprintf("最大並列実行数 (デフォルト: %d)", scraper.DefaultMaxConcurrency))
}
