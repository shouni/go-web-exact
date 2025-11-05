package cmd

import (
	"log"
	"time"

	"github.com/shouni/go-cli-base"
	"github.com/shouni/go-http-kit/pkg/httpkit"
	"github.com/spf13/cobra"
)

const (
	appName           = "webparse"
	defaultTimeoutSec = 10 // 秒
	defaultMaxRetries = 5  // デフォルトのリトライ回数
)

// GlobalFlags はこのアプリケーション固有の永続フラグを保持
// clibase.Flags は clibase 共通フラグ（Verbose, ConfigFile）を保持
type AppFlags struct {
	// 💡 修正点1: コメントを簡潔に修正
	TimeoutSec int // --timeout タイムアウト
	MaxRetries int
}

var Flags AppFlags // アプリケーション固有フラグにアクセスするためのグローバル変数
var globalFetcher httpkit.Fetcher

// --- アプリケーション固有のカスタム関数 ---

// addAppPersistentFlags は、アプリケーション固有の永続フラグをルートコマンドに追加します。
func addAppPersistentFlags(rootCmd *cobra.Command) {
	rootCmd.PersistentFlags().IntVar(
		&Flags.TimeoutSec,
		"timeout",
		defaultTimeoutSec,
		"HTTPリクエストのタイムアウト時間（秒）",
	)

	// 💡 修正点2: 不要なコメントを削除
	rootCmd.PersistentFlags().IntVar(
		&Flags.MaxRetries,
		"max-retries",
		defaultMaxRetries,
		"HTTPリクエストのリトライ最大回数",
	)
}

// initAppPreRunE は、clibase共通処理の後に実行される、アプリケーション固有のPersistentPreRunEです。
func initAppPreRunE(cmd *cobra.Command, args []string) error {
	timeout := time.Duration(Flags.TimeoutSec) * time.Second

	if clibase.Flags.Verbose {
		log.Printf("HTTPクライアントのタイムアウトを設定しました (Timeout: %s)。", timeout)
		log.Printf("HTTPクライアントのリトライ回数を設定しました (MaxRetries: %d)。", Flags.MaxRetries)
	}

	// 💡 修正点3: 不要なコメントを削除
	globalFetcher = httpkit.New(timeout, httpkit.WithMaxRetries(Flags.MaxRetries))

	return nil
}

// GetGlobalFetcher は、初期化されたフェッチャーを返す関数
// 💡 アーキテクチャに関する指摘: DIを推奨。clibaseの制約上、現状はグローバル関数を使用するが、
// 理想的には、このフェッチャーをコンテキストまたはコマンド構造体を介して渡すべき。
func GetGlobalFetcher() httpkit.Fetcher {
	return globalFetcher
}

// --- エントリポイント ---

// Execute は、rootCmd を実行するメイン関数です。
func Execute() {
	clibase.Execute(
		appName,
		addAppPersistentFlags,
		initAppPreRunE,
		extractCmd,
	)
}
