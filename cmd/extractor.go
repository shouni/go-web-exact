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

// コマンドラインフラグ変数を定義
var rawUrl string

// NOTE: 以前このファイルにあった定数 defaultOverallTimeoutIfClientTimeoutIsZero は削除されました。
// 代わりに、cmd/root.go で定義された DefaultOverallTimeout を使用します。

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

// ensureScheme は、URLのスキームが存在しない場合に https:// を補完します。
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
		// 既存のスキームを尊重
		return rawURL, nil
	}

	// 3. スキームがない場合、HTTPSをデフォルトとして付与
	return "https://" + rawURL, nil
}

var extractorcmd = &cobra.Command{
	Use:   "extract",
	Short: "指定されたURLまたは標準入力からWebコンテンツのテキストを取得します",
	Long:  `指定されたURLまたは標準入力からWebコンテンツのテキストを取得します。`,

	// 位置引数は取らない設定
	Args: cobra.NoArgs,

	RunE: func(cmd *cobra.Command, args []string) error {

		// overallTimeout の設定: クライアントタイムアウト (Flags.TimeoutSec) の2倍を全体のタイムアウトとします。
		overallTimeout := time.Duration(Flags.TimeoutSec) * 2 * time.Second
		if Flags.TimeoutSec == 0 {
			overallTimeout = DefaultOverallTimeout
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

		// 2. URLのスキーム補完とバリデーション
		processedURL, err := ensureScheme(urlToProcess)
		if err != nil {
			return fmt.Errorf("URLスキームの処理エラー: %w", err)
		}
		log.Printf("処理対象URL: %s (全体タイムアウト: %s)\n", processedURL, overallTimeout)

		// 3. 依存性の初期化
		// cmd/root.go で初期化された共有フェッチャーを使用。
		fetcher := GetGlobalFetcher()
		if fetcher == nil {
			return fmt.Errorf("HTTPクライアントの取得に失敗しました")
		}

		// ユーザーの記憶にある extract パッケージの NewExtractor を利用
		extractor, err := extract.NewExtractor(fetcher)
		if err != nil {
			return fmt.Errorf("Extractorの初期化エラー: %w", err)
		}

		// 4. メインロジックの実行
		text, isBodyExtracted, err := runExtractionPipeline(processedURL, extractor, overallTimeout)
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
	extractorcmd.Flags().StringVarP(&rawUrl, "url", "u", "", "抽出対象のURL")
}
