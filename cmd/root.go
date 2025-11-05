package cmd

import (
	"log"
	"time"

	"github.com/shouni/go-cli-base"
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

// --- アプリケーション固有のカスタム関数 ---

// addAppPersistentFlags は、アプリケーション固有の永続フラグをルートコマンドに追加します。
func addAppPersistentFlags(rootCmd *cobra.Command) {
	// 💡 Flags.TimeoutSec にフラグの値をバインドします
	rootCmd.PersistentFlags().IntVar(
		&Flags.TimeoutSec, // 変数のポインタを渡す
		"timeout",         // フラグ名
		defaultTimeoutSec, // デフォルト値
		"HTTPリクエストのタイムアウト時間（秒）", // 説明
	)
}

// initAppPreRunE は、clibase共通処理の後に実行される、アプリケーション固有のPersistentPreRunEです。
func initAppPreRunE(cmd *cobra.Command, args []string) error {
	// 修正: Flags構造体の値を使用してタイムアウトdurationを計算し、ログに出力します。
	timeout := time.Duration(Flags.TimeoutSec) * time.Second

	// clibase共通処理（Verboseなど）は clibase 側で既に実行されている
	// clibaseのVerboseフラグと連携したロギング
	if clibase.Flags.Verbose {
		// 修正: 未定義変数 'timeout' を、計算した time.Duration の 'timeout' に置き換え
		log.Printf("HTTPクライアントのタイムアウトを設定しました (Timeout: %s)。", timeout)
	}

	// 💡 ここで、他のコマンド（例: extractCmd）で使えるように、
	// タイムアウト設定済みの http.Client などを初期化・グローバル変数に格納するロジックを通常追加します。
	// 例: globalClient = httpkit.NewClient(httpkit.Config{Timeout: timeout})

	return nil
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
		// 💡 ユーザーの記憶にある extract パッケージがウェブコンテンツ抽出に関わるため、
		// サブコマンドとして `extractCmd` が存在すると仮定します。
		extractCmd, // 既存のサブコマンド
	)
}
