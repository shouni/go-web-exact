package cmd

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net/url"
	"os"
	"time"

	"github.com/shouni/go-web-exact/v2/pkg/extract"
	"github.com/spf13/cobra"
)

var rawUrl string

// runExtractionPipeline は、Webコンテンツの抽出を実行するメインロジックです。
// Goの慣習に従い、エラーを最後の戻り値にします。
func runExtractionPipeline(rawURL string, extractor *extract.Extractor, overallTimeout time.Duration) (text string, isBodyExtracted bool, err error) {
	// 1. 全体処理のコンテキストを設定
	// 💡 overallTimeout は、URLパース、ネットワーク処理（リトライ含む）、HTMLパース全体をカバーする時間
	ctx, cancel := context.WithTimeout(context.Background(), overallTimeout)
	defer cancel()

	// 2. 抽出の実行
	// Context付きで extractor.FetchAndExtractText を呼び出し、タイムアウトを伝播させる
	text, isBodyExtracted, err = extractor.FetchAndExtractText(ctx, rawURL)
	if err != nil {
		// エラーのラッピング
		return "", false, fmt.Errorf("コンテンツ抽出エラー (URL: %s): %w", rawURL, err)
	}

	return text, isBodyExtracted, nil
}

// ensureScheme は、URLのスキームが存在しない場合に https:// または http:// を補完します。
// スキームが既に存在する場合は、それが http または https であるかをチェックします。
func ensureScheme(rawURL string) (string, error) {
	// 1. まず現在のURLをパース
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("URLのパースエラー: %w", err)
	}

	// 2. スキームが既に存在する場合のチェック
	if parsedURL.Scheme != "" {
		if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
			return "", fmt.Errorf("無効なURLスキームです。httpまたはhttpsを指定してください: %s", rawURL)
		}
		return rawURL, nil
	}

	// 3. スキームがない場合、HTTPSを優先的に試す
	return "https://" + rawURL, nil
}

var extractCmd = &cobra.Command{
	Use:   "extract [URL]",
	Short: "指定されたURLまたは標準入力からWebコンテンツのテキストを取得します",
	Long:  `指定されたURLまたは標準入力からWebコンテンツのテキストを取得します。`,
	RunE: func(cmd *cobra.Command, args []string) error {

		// 💡 修正点1: clientTimeout (30秒) のハードコードを削除。
		// HTTPリクエストのタイムアウトは GetGlobalFetcher() から取得されるクライアントが保持します。

		// 💡 overallTimeout の設定: クライアントタイムアウトとは別に、抽出プロセス全体のタイムアウトを設定します。
		// ここでは、root.goで設定されたクライアントタイムアウト (Flags.TimeoutSec) の2倍を全体のタイムアウトとします。
		// タイムアウトを `time.Duration` に変換します。
		overallTimeout := time.Duration(Flags.TimeoutSec) * 2 * time.Second
		if Flags.TimeoutSec == 0 {
			// 0秒が設定された場合の防御的な設定
			overallTimeout = 20 * time.Second
		}

		// 1. 処理対象URLの決定 (フラグ優先)
		urlToProcess := rawUrl
		if urlToProcess == "" {
			log.Println("URLが指定されていないため、標準入力からURLを読み込みます...")
			scanner := bufio.NewScanner(os.Stdin)
			fmt.Print("処理するURLを入力してください: ")

			if !scanner.Scan() {
				if err := scanner.Err(); err != nil {
					return fmt.Errorf("標準入力の読み取りエラー: %w", err)
				}
				return fmt.Errorf("URLが入力されていません")
			}
			urlToProcess = scanner.Text()
		}

		// 2. URLのスキーム補完とバリデーション (ヘルパー関数に分離)
		processedURL, err := ensureScheme(urlToProcess)
		if err != nil {
			return fmt.Errorf("URLスキームの処理エラー: %w", err)
		}
		log.Printf("処理対象URL: %s (全体タイムアウト: %s)\n", processedURL, overallTimeout)

		// 3. 依存性の初期化 (DIコンテナの役割)
		// 💡 修正点2: cmd/root.go で初期化された共有フェッチャーを使用。
		// これにより、ユーザー指定の --timeout が反映されます。
		fetcher := GetGlobalFetcher()
		if fetcher == nil {
			// GetGlobalFetcherがnilを返すことは通常ありませんが、念のためチェック
			return fmt.Errorf("HTTPクライアントが初期化されていません。rootコマンドのPreRunを確認してください")
		}

		// ユーザーの記憶にある extract パッケージの NewExtractor を利用
		extractor, err := extract.NewExtractor(fetcher)
		if err != nil {
			return fmt.Errorf("Extractorの初期化エラー: %w", err)
		}

		// 4. メインロジックの実行 (ヘルパー関数を呼び出し)
		text, isBodyExtracted, err := runExtractionPipeline(processedURL, extractor, overallTimeout)
		if err != nil {
			return fmt.Errorf("コンテンツ抽出パイプラインの実行エラー: %w", err)
		}

		// 5. 結果の出力
		if !isBodyExtracted {
			fmt.Printf("本文は見つかりませんでしたが、タイトルを取得しました:\n%s\n", text)
		} else {
			fmt.Println("--- 抽出された本文 ---")
			fmt.Println(text)
			fmt.Println("-----------------------")
		}

		return nil
	},
}

func init() {
	// rawUrl 変数にフラグのポインタをバインドします
	extractCmd.Flags().StringVarP(&rawUrl, "url", "u", "", "抽出対象のURL")
}
