package main

import (
	"bufio"
	"fmt"
	"log"
	"net/url"
	"os"

	"github.com/shouni/go-web-exact/v2/internal/pipeline"
)

func main() {
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

	// 2. URLのバリデーション (I/Oに近い責務)
	if rawURL == "" {
		log.Fatalf("無効なURLが入力されました。")
	}
	parsedURL, err := url.Parse(rawURL)
	if err != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") {
		log.Fatalf("無効なURLスキームです: %s", rawURL)
	}
	fmt.Printf("入力されたURL: %s\n", rawURL)

	// 3. メインロジックの実行
	text, hasBody, err := pipeline.ExtractURLContent(rawURL)

	if err != nil {
		log.Fatalf("処理中にエラーが発生しました: %v", err) // 処理実行エラーを報告
	}

	// 4. 結果の出力 (main.goの責務)
	if !hasBody {
		fmt.Printf("本文は見つかりませんでしたが、タイトルを取得しました:\n%s\n", text)
	} else {
		fmt.Println("--- 抽出された本文 ---")
		fmt.Println(text)
		fmt.Println("-----------------------")
	}
}
