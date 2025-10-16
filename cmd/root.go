package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/shouni/go-web-exact/pkg/httpclient"
	"github.com/shouni/go-web-exact/pkg/web"
)

// グローバル定数
const separator = "=============================================="

// グローバル変数: コマンドラインフラグの値を保持
var (
	timeout int
	urlArg  string
)

// rootCmd はアプリケーションのメインコマンドです
var rootCmd = &cobra.Command{
	Use:   "web-exact [URL]",
	Short: "指定されたURLからクリーンな記事本文を正確に抽出します。",
	Long: `web-exact は、AI/LLMへの入力のために、ウェブページから広告やナビゲーションを除去し、
整理された記事本文を抽出するためのCLIツールです。

利用例:
  web-exact https://example.com/article
  web-exact -u https://example.com/article -t 15`,

	// 実行されるメインロジック (エラーハンドリングのため RunE を使用)
	RunE: func(cmd *cobra.Command, args []string) error {

		// 1. 引数（URL）の確定
		if urlArg == "" && len(args) > 0 {
			urlArg = args[0]
		}
		if urlArg == "" {
			return fmt.Errorf("致命的エラー: URLがコマンドラインから提供されませんでした。Args検証ロジックを確認してください。")
		}

		// 2. タイムアウト設定とコンテキスト作成
		timeoutDuration := time.Duration(timeout) * time.Second
		ctx, cancel := context.WithTimeout(context.Background(), timeoutDuration)
		defer cancel()

		// 3. HTTPクライアントのインスタンスを作成 (リトライ機能付き)
		fetcherClient := httpclient.New(timeoutDuration)

		// 4. Extractor のインスタンスを作成 (依存性の注入)
		extractor := web.NewExtractor(fetcherClient)

		// 5. 抽出ロジックの実行
		fmt.Printf("URL: %s からコンテンツを抽出中 (Timeout: %d秒)...\n", urlArg, timeout)

		text, hasBody, err := extractor.FetchAndExtractText(urlArg, ctx)

		if err != nil {
			return fmt.Errorf("抽出処理中にエラーが発生しました: %w", err)
		}

		// 6. 結果の出力
		fmt.Println("\n" + separator)
		if !hasBody {
			fmt.Println("|| 抽出結果 (本文として認識できる内容なし) ||")
		} else {
			fmt.Println("|| 抽出結果 (本文あり) ||")
		}
		fmt.Println(separator)
		fmt.Println(text)
		fmt.Println(separator)

		return nil // 正常終了
	},

	// 引数検証のカスタムロジック
	Args: func(cmd *cobra.Command, args []string) error {
		isURLFlagChanged := cmd.Flags().Changed("url")

		if isURLFlagChanged {
			if len(args) > 0 {
				return fmt.Errorf("エラー: --url フラグが指定されている場合、位置引数 (URL) は不要です。")
			}
			return nil // フラグで指定されているのでOK
		}

		// フラグがない場合は位置引数が必須。
		// cobra.ExactArgs(1) を使って、引数がちょうど1つであることを検証
		return cobra.ExactArgs(1)(cmd, args)
	},
}

// Execute はルートコマンドを実行します。cmd/main.go から呼び出されます。
func Execute() error {
	return rootCmd.Execute()
}

// init() はアプリケーション起動時に自動的に実行され、フラグを設定します。
func init() {
	rootCmd.PersistentFlags().IntVarP(&timeout, "timeout", "t", 30, "HTTPリクエストのタイムアウト時間 (秒)")
	rootCmd.Flags().StringVarP(&urlArg, "url", "u", "", "抽出対象のURL (位置引数として指定可)")
}
