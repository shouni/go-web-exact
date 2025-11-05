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
)

// GlobalFlags はこのアプリケーション固有の永続フラグを保持
// clibase.Flags は clibase 共通フラグ（Verbose, ConfigFile）を保持
type AppFlags struct {
	TimeoutSec int // --timeout タイムアウト
}

var Flags AppFlags // アプリケーション固有フラグにアクセスするためのグローバル変数

// 💡 修正点1: パッケージレベルで httpkit.Fetcher インターフェースを保持する変数を定義
var globalFetcher httpkit.Fetcher

// --- アプリケーション固有のカスタム関数 ---

// addAppPersistentFlags は、アプリケーション固有の永続フラグをルートコマンドに追加します。
func addAppPersistentFlags(rootCmd *cobra.Command) {
	// Flags.TimeoutSec にフラグの値をバインドします
	rootCmd.PersistentFlags().IntVar(
		&Flags.TimeoutSec, // 変数のポインタを渡す
		"timeout",         // フラグ名
		defaultTimeoutSec, // デフォルト値
		"HTTPリクエストのタイムアウト時間（秒）", // 説明
	)
}

// initAppPreRunE は、clibase共通処理の後に実行される、アプリケーション固有のPersistentPreRunEです。
func initAppPreRunE(cmd *cobra.Command, args []string) error {
	timeout := time.Duration(Flags.TimeoutSec) * time.Second

	// clibase共通処理（Verboseなど）は clibase 側で既に実行されている
	// clibaseのVerboseフラグと連携したロギング
	if clibase.Flags.Verbose {
		log.Printf("HTTPクライアントのタイムアウトを設定しました (Timeout: %s)。", timeout)
	}

	// 💡 修正点2: PersistentPreRunE内で、グローバルな HTTP クライアント (Fetcher) を初期化
	// root コマンド実行前に一度だけ初期化されるため、全てのサブコマンドで共有されます。
	// クライアントごとのタイムアウトとして Flags.TimeoutSec を使用します。
	// リトライはハードコードされた5回とします。
	globalFetcher = httpkit.New(timeout, httpkit.WithMaxRetries(5))

	return nil
}

// 💡 修正点3: 初期化された httpkit.Fetcher を返すエクスポートされた関数
// 他のサブコマンド（例：extractCmd）がこの共通依存性を取得するために使用します。
func GetGlobalFetcher() httpkit.Fetcher {
	return globalFetcher
}

// --- エントリポイント ---

// Execute は、rootCmd を実行するメイン関数です。
func Execute() {
	// ここで clibase.Execute を使用して、ルートコマンドの構築と実行を委譲します。
	// Execute(アプリ名, カスタムフラグ追加関数, PersistentPreRunE関数, サブコマンド...)
	clibase.Execute(
		appName,
		addAppPersistentFlags,
		initAppPreRunE,
		extractCmd, // 既存のサブコマンド
	)
}
