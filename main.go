package main

import (
	// 💡 修正 1: 標準ライブラリをグループ化し、最初に配置
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
// 💡 修正 2: overallTimeout を引数として受け取るように変更
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

func main() {
	const overallTimeout = 60 * time.Second
	const clientTimeout = 30 * time.Second

	// 1. 標準入力からURLを読み取る (I/Oの責務)
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Print("処理するURLを入力してください: ")

	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			log.Fatalf("標準入力の読み取りエラー: %v", err)
		}
		log.Fatalf("URLが入力されていません。")
	}
	rawURL := scanner.Text()

	// 2. URLのバリデーションとスキーム補完
	if rawURL == "" {
		log.Fatalf("無効なURLが入力されました。")
	}

	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		log.Fatalf("URLのパースエラー: %v", err)
	}

	// スキームがない場合、http:// を補完するロジックを追加
	// 💡 修正 3: ". " の条件を削除し、スキームが空の場合のみ補完する
	if parsedURL.Scheme == "" {
		rawURL = "http://" + rawURL
		parsedURL, err = url.Parse(rawURL)
		if err != nil {
			log.Fatalf("URLのパースエラー (スキーム補完後): %v", err)
		}
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		log.Fatalf("無効なURLスキームです。httpまたはhttpsを指定してください: %s", rawURL)
	}
	fmt.Printf("入力されたURL: %s\n", rawURL)

	// 3. 依存性の初期化 (DIコンテナの役割)
	// clientTimeout を使用して fetcher を初期化
	fetcher := httpkit.New(clientTimeout, httpkit.WithMaxRetries(2))
	extractor, err := extract.NewExtractor(fetcher)
	if err != nil {
		log.Fatalf("Extractorの初期化エラー: %v", err)
	}

	// 4. メインロジックの実行 (ヘルパー関数を呼び出し)
	// 💡 修正 2: overallTimeout を引数として渡す
	text, hasBody, err := runExtractionPipeline(rawURL, extractor, overallTimeout)

	if err != nil {
		log.Fatalf("処理中にエラーが発生しました: %v", err)
	}

	// 5. 結果の出力
	if !hasBody {
		fmt.Printf("本文は見つかりませんでしたが、タイトルを取得しました:\n%s\n", text)
	} else {
		fmt.Println("--- 抽出された本文 ---")
		fmt.Println(text)
		fmt.Println("-----------------------")
	}
}
