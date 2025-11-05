package cmd

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/shouni/go-web-exact/v2/pkg/feed"
	"github.com/spf13/cobra"
)

// フィード解析の全体処理のタイムアウト係数
const overallFeedTimeoutFactor = 2

// コマンドラインフラグ変数
var (
	feedURL string // -u, --url フラグで受け取るフィードURL
)

// runParsePipeline はフィードの取得と解析を実行するメインロジックです。
func runParsePipeline(feedURL string, fetcher feed.Fetcher) error {

	// 1. 全体タイムアウトの設定
	clientTimeout := time.Duration(Flags.TimeoutSec) * time.Second
	if Flags.TimeoutSec == 0 {
		clientTimeout = defaultTimeoutSec * time.Second
	}
	overallTimeout := clientTimeout * overallFeedTimeoutFactor

	// 2. コンテキストの設定
	ctx, cancel := context.WithTimeout(context.Background(), overallTimeout)
	defer cancel()

	log.Printf("フィード解析開始 (URL: %s, 全体タイムアウト: %s)\n", feedURL, overallTimeout)

	// 3. フィードパーサーの初期化
	parser := feed.NewParser(fetcher)

	// 4. フィードの取得とパースを実行
	rssFeed, err := parser.FetchAndParse(ctx, feedURL)
	if err != nil {
		return fmt.Errorf("フィードのパース失敗: %w", err)
	}

	// 5. 結果の出力
	fmt.Printf("\n--- フィード解析結果 ---\n")
	fmt.Printf("タイトル: %s\n", rssFeed.Title)
	fmt.Printf("URL: %s\n", rssFeed.Link)

	// 💡 修正: UpdatedParsed が nil でないかチェック (パニック対策)
	if rssFeed.UpdatedParsed != nil {
		fmt.Printf("更新日時: %s\n", rssFeed.UpdatedParsed.Local().Format("2006/01/02 15:04:05"))
	} else {
		// 更新日時がない場合はその旨を出力
		fmt.Printf("更新日時: (情報なし)\n")
	}

	fmt.Printf("記事数: %d\n", len(rssFeed.Items))
	fmt.Println("----------------------")

	// 記事リストの表示
	for i, item := range rssFeed.Items {
		fmt.Printf("[%d] %s\n", i+1, item.Title)
		fmt.Printf("    - リンク: %s\n", item.Link)

		// 💡 記事の公開日時も nil チェックを追加し、堅牢性を向上
		if item.PublishedParsed != nil {
			fmt.Printf("    - 公開: %s\n", item.PublishedParsed.Local().Format("2006/01/02 15:04:05"))
		}
	}
	fmt.Println("----------------------")

	return nil
}

var parseCmd = &cobra.Command{
	Use:   "parse",
	Short: "RSS/Atomフィードを取得・解析し、タイトルと記事を一覧表示します",
	Long:  `指定されたフィードURLからコンテンツを取得し、フィードのタイトルや記事のリンクなどを標準出力に出力します。`,
	Args:  cobra.NoArgs,

	RunE: func(cmd *cobra.Command, args []string) error {

		// 1. 依存性の初期化 (Fetcher)
		fetcher := GetGlobalFetcher()
		if fetcher == nil {
			return fmt.Errorf("HTTPクライアントの取得に失敗しました")
		}

		// 2. URLのバリデーションとスキーム補完
		if feedURL == "" {
			return fmt.Errorf("フィードURLを指定してください (--urlまたは-u)")
		}

		processedURL, err := ensureScheme(feedURL)
		if err != nil {
			return fmt.Errorf("URLスキームの処理エラー: %w", err)
		}

		// 3. メインロジックの実行
		return runParsePipeline(processedURL, fetcher)
	},
}

func init() {
	// -u, --url フラグの定義
	parseCmd.Flags().StringVarP(&feedURL, "url", "u", "", "解析対象のフィードURL (RSS/Atom)")
	parseCmd.MarkFlagRequired("url")
}
