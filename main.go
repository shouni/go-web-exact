package main

import (
	"log"
	"os"

	"github.com/shouni/go-web-exact/cmd"
)

func main() {
	// ログのタイムスタンプなどを非表示にする設定を維持
	log.SetFlags(0)

	// ★ 修正: cmd.Execute() から返されるエラーを処理し、非ゼロの終了コードで終了
	if err := cmd.Execute(); err != nil {
		// Cobraが出力するエラーメッセージに加えて、独自のエラーをstderrに出力することも可能
		// fmt.Fprintf(os.Stderr, "エラー: %v\n", err)
		os.Exit(1) // エラーを示す終了コード
	}
}
