package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net/url"
	"os"
	"time"

	"github.com/shouni/go-http-kit/pkg/httpkit"
	"github.com/shouni/go-web-exact/v2/pkg/extract"
)

// runExtractionPipeline は、Webコンテンツの抽出を実行するメインロジックです。
func runExtractionPipeline(rawURL string, extractor *extract.Extractor, overallTimeout time.Duration) (text string, hasBody bool, err error) {
	// 1. 全体処理のコンテキストを設定
	ctx, cancel := context.WithTimeout(context.Background(), overallTimeout)
	defer cancel()

	// 2. 抽出の実行
	text, hasBody, err = extractor.FetchAndExtractText(rawURL, ctx)
	if err != nil {
		return "", false, fmt.Errorf("コンテンツ抽出エラー: %w", err)
	}

	return text, hasBody, nil
}

// run は、アプリケーションの主要なロジックをカプセル化し、エラーを返します。
// これにより、main関数がエラーハンドリングに専念できます。
func run() error {
	const overallTimeout = 60 * time.Second
	const clientTimeout = 30 * time.Second

	// 1. 標準入力からURLを読み取る (I/Oの責務)
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Print("処理するURLを入力してください: ")

	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return fmt.Errorf("標準入力の読み取りエラー: %w", err)
		}
		return fmt.Errorf("URLが入力されていません")
	}
	rawURL := scanner.Text()

	// 2. URLのバリデーションとスキーム補完
	if rawURL == "" {
		return fmt.Errorf("無効なURLが入力されました")
	}

	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("URLのパースエラー: %w", err)
	}

	// スキームがない場合、http:// を補完するロジックを追加
	if parsedURL.Scheme == "" {
		rawURL = "http://" + rawURL
		parsedURL, err = url.Parse(rawURL)
		if err != nil {
			return fmt.Errorf("URLのパースエラー (スキーム補完後): %w", err)
		}
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("無効なURLスキームです。httpまたはhttpsを指定してください: %s", rawURL)
	}
	fmt.Printf("入力されたURL: %s\n", rawURL)

	// 3. 依存性の初期化 (DIコンテナの役割)
	fetcher := httpkit.New(clientTimeout, httpkit.WithMaxRetries(2))
	extractor, err := extract.NewExtractor(fetcher)
	if err != nil {
		return fmt.Errorf("Extractorの初期化エラー: %w", err)
	}

	// 4. メインロジックの実行 (ヘルパー関数を呼び出し)
	text, hasBody, err := runExtractionPipeline(rawURL, extractor, overallTimeout)
	if err != nil {
		return err // runExtractionPipelineのエラーをそのまま返す
	}

	// 5. 結果の出力
	if !hasBody {
		fmt.Printf("本文は見つかりませんでしたが、タイトルを取得しました:\n%s\n", text)
	} else {
		fmt.Println("--- 抽出された本文 ---")
		fmt.Println(text)
		fmt.Println("-----------------------")
	}

	return nil
}

// main 関数は、run 関数を実行し、エラーが発生した場合は log.Fatalf でアプリケーションを終了させます。
func main() {
	if err := run(); err != nil {
		// エラーハンドリングを一元化
		log.Fatalf("アプリケーションエラー: %v", err)
	}
}
