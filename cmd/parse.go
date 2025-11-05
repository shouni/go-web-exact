package cmd

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/mmcdole/gofeed"
	"github.com/shouni/go-http-kit/pkg/httpkit"
	"github.com/spf13/cobra"

	// ユーザーの記憶にある package feed (parser.go) を利用します
	"go-web-exact/pkg/feed"
)

// フィードURLを保持するフラグ変数
var feedURL string

// フィードの全体処理のタイムアウト設定 (extractCmdと統一)
const overallFeedTimeoutFactor = 2 // クライアントタイムアウトの2倍

// runParsePipeline は、フィードの取得とパースを実行するメインロジックです。
func runParsePipeline(url string, parser *feed.Parser, overallTimeout time.Duration) (*gofeed.Feed, error) {
	// 1. 全体処理のコンテキストを設定
	ctx, cancel := context.WithTimeout(context.Background(), overallTimeout)
	defer cancel()

	// 2. 抽出の実行
	parsedFeed, err := parser.FetchAndParse(ctx, url)
	if err != nil {
		// エラーのラッピング
		return nil, fmt.Errorf("フィードの取得およびパースエラー (URL: %s): %w", url, err)
	}

	return parsedFeed, nil
}

var parseCmd = &cobra.Command{
	Use:   "parse",
	Short: "RSS/Atomフィードを取得・解析し、タイトルと記事を一覧表示します",
	Long:  `指定されたURLからRSSまたはAtomフィードを取得し、その内容（フィードタイトル、記事タイトル、URL）を整形して表示します。`,

	// 💡 修正点: 位置引数を禁止する設定を追加 (ユーザーエラーの解決)
	Args: cobra.NoArgs,

	RunE: func(cmd *cobra.Command, args []string) error {

		// Flags.TimeoutSec は cmd/root.go で定義されています
		// 全体タイムアウトを設定: クライアントタイムアウトの2倍 (extractCmdと統一)
		overallTimeout := time.Duration(Flags.TimeoutSec*overallFeedTimeoutFactor) * time.Second
		if Flags.TimeoutSec == 0 {
			// extractCmdの定数を流用
			overallTimeout = defaultOverallTimeoutIfClientTimeoutIsZero
		}

		log.Printf("処理対象フィードURL: %s (全体タイムアウト: %s)\n", feedURL, overallTimeout)

		// 1. 依存性の初期化
		// cmd/root.go で初期化された共有フェッチャーを使用
		fetcher := GetGlobalFetcher()
		if fetcher == nil {
			return fmt.Errorf("HTTPクライアントが初期化されていません。rootコマンドのPreRunを確認してください")
		}

		// Fetcherインターフェースを *httpkit.Client の実装にダウンキャストします。
		// NewParserが *httpkit.Client を受け取るため。
		client, ok := fetcher.(*httpkit.Client)
		if !ok {
			return fmt.Errorf("予期しないフェッチャーの実装です: %T", fetcher)
		}

		// ユーザーの記憶にある package feed の NewParser を利用
		parser := feed.NewParser(client)

		// 2. メインロジックの実行
		parsedFeed, err := runParsePipeline(feedURL, parser, overallTimeout)
		if err != nil {
			return fmt.Errorf("フィード解析パイプラインの実行エラー: %w", err)
		}

		// 3. 結果の出力
		fmt.Printf("--- フィード解析結果 ---\n")
		fmt.Printf("フィードタイトル: %s\n", parsedFeed.Title)
		if parsedFeed.Link != "" {
			fmt.Printf("リンク: %s\n", parsedFeed.Link)
		}
		fmt.Printf("合計記事数: %d\n", len(parsedFeed.Items))
		fmt.Println("-----------------------")

		for i, item := range parsedFeed.Items {
			fmt.Printf("[%d] %s\n", i+1, item.Title)
			fmt.Printf("    URL: %s\n", item.Link)
			if item.PublishedParsed != nil {
				fmt.Printf("    公開日: %s\n", item.PublishedParsed.Local().Format("2006-01-02 15:04:05"))
			}
		}

		return nil
	},
}

func init() {
	// サブコマンド固有のフラグ定義
	parseCmd.Flags().StringVarP(&feedURL, "url", "u", "", "解析対象のフィード (RSS/Atom) URL")

	// URLフラグを必須にする
	parseCmd.MarkFlagRequired("url")
}
