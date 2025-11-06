package cmd

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/shouni/go-http-kit/pkg/httpkit"
	"github.com/shouni/go-web-exact/v2/pkg/extract"
	"github.com/spf13/cobra"
)

// runExtractionPipeline は、Webコンテンツの抽出を実行するメインロジックです。
func runExtractionPipeline(rawURL string, extractor *extract.Extractor, overallTimeout time.Duration) (text string, isBodyExtracted bool, err error) {
	// 1. 全体処理のコンテキストを設定
	ctx, cancel := context.WithTimeout(context.Background(), overallTimeout)
	defer cancel()

	// 2. 抽出の実行
	text, isBodyExtracted, err = extractor.FetchAndExtractText(ctx, rawURL)
	if err != nil {
		// エラーのラッピング
		return "", false, fmt.Errorf("コンテンツ抽出エラー (URL: %s): %w", rawURL, err)
	}

	return text, isBodyExtracted, nil
}

var extracCommand = &cobra.Command{
	Use:   "extract",
	Short: "指定されたURLまたは標準入力からWebコンテンツのテキストを取得します",
	Long:  `指定されたURLまたは標準入力からWebコンテンツのテキストを取得します。`,

	// 位置引数は取らない設定
	Args: cobra.NoArgs,

	RunE: func(cmd *cobra.Command, args []string) error {

		// 1. 処理対象URLの決定 (フラグ優先)
		urlToProcess := feedURL
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

		// 2. URLのスキーム補完とバリデーション
		processedURL, err := ensureScheme(urlToProcess)
		if err != nil {
			return fmt.Errorf("URLスキームの処理エラー: %w", err)
		}
		log.Printf("処理対象URL: %s (全体タイムアウト: %s)\n", processedURL, defaultTimeoutSec)

		// 3. 依存性の初期化
		fetcher := httpkit.New(defaultMaxRetries)
		if fetcher == nil {
			return fmt.Errorf("HTTPクライアントの取得に失敗しました")
		}

		extractor, err := extract.NewExtractor(fetcher)
		if err != nil {
			return fmt.Errorf("Extractorの初期化エラー: %w", err)
		}

		// 4. メインロジックの実行
		text, isBodyExtracted, err := runExtractionPipeline(processedURL, extractor, defaultTimeoutSec)
		if err != nil {
			return fmt.Errorf("コンテンツ抽出パイプラインの実行エラー (URL: %s): %w", processedURL, err)
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
	extracCommand.Flags().StringVarP(&feedURL, "url", "u", "https://github.com/shouni/go-web-exact", "抽出対象のURL")
}
