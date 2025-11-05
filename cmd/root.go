package cmd

import (
	"log"
	"os"
	"time"

	"github.com/shouni/go-cli-base"
	"github.com/shouni/go-http-kit/pkg/httpkit"
	"github.com/spf13/cobra"
)

const (
	appName           = "webparse"
	defaultTimeoutSec = 10 // 秒
	defaultMaxRetries = 5  // デフォルトのリトライ回数

	// 💡 修正点1: parseCmdで参照される共通定数を定義
	DefaultOverallTimeoutIfClientTimeoutIsZero = 20 * time.Second
)

// GlobalFlags はこのアプリケーション固有の永続フラグを保持
// clibase.Flags は clibase 共通フラグ（Verbose, ConfigFile）を保持
type AppFlags struct {
	TimeoutSec int // --timeout タイムアウト
	MaxRetries int
}

var Flags AppFlags // アプリケーション固有フラグにアクセスするためのグローバル変数
var globalFetcher httpkit.Fetcher

// 💡 修正点2: ルートコマンドを定義
var rootCmd = &cobra.Command{
	Use:   "web-exact",
	Short: "Webコンテンツ抽出・フィード解析ツール",
	Long:  `Webコンテンツの抽出（extract）またはRSS/Atomフィードの解析（parse）を実行します。`,

	// 💡 修正点3: ルートコマンドが引数を取らないことを明示 (引数エラーを解消)
	Args: cobra.NoArgs,
}

// --- アプリケーション固有のカスタム関数 ---

// addAppPersistentFlags は、アプリケーション固有の永続フラグをルートコマンドに追加します。
func addAppPersistentFlags(rootCmd *cobra.Command) {
	rootCmd.PersistentFlags().IntVar(
		&Flags.TimeoutSec,
		"timeout",
		defaultTimeoutSec,
		"HTTPリクエストのタイムアウト時間（秒）",
	)

	// 💡 修正点4: 不要なコメントを削除
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

	// 💡 修正点5: 不要なコメントを削除
	globalFetcher = httpkit.New(timeout, httpkit.WithMaxRetries(Flags.MaxRetries))

	return nil
}

// GetGlobalFetcher は、初期化されたフェッチャーを返す関数
// 💡 アーキテクチャに関する指摘: DIを推奨。clibaseの制約上、現状はグローバル関数を使用するが、
// 理想的には、このフェッチャーをコンテキストまたはコマンド構造体を介して渡すべき。
func GetGlobalFetcher() httpkit.Fetcher {
	return globalFetcher
}

// 💡 修正点6: init() 関数でサブコマンドをルートコマンドに追加
func init() {
	// extractCmd と parseCmd は別ファイルで var として定義されていると仮定
	rootCmd.AddCommand(extractorcmd)
	rootCmd.AddCommand(parseCmd)
}

// --- エントリポイント ---

// Execute は、rootCmd を実行するメイン関数です。
func Execute() {
	// グローバルフラグの設定 (init() で AddCommand が実行された後に実行する必要がある)
	addAppPersistentFlags(rootCmd)

	// PersistentPreRunEの設定 (DIの初期化)
	rootCmd.PersistentPreRunE = initAppPreRunE

	if err := rootCmd.Execute(); err != nil {
		// エラーメッセージは Cobra が処理するため、os.Exit(1) のみで十分
		os.Exit(1)
	}
}
