package cmd

import (
	"log"
	"time"

	clibase "github.com/shouni/go-cli-base"
	"github.com/spf13/cobra"
)

// --- グローバル定数 ---

const (
	appName           = "web-exact" // アプリケーション名を修正
	defaultTimeoutSec = 10          // 秒
	defaultMaxRetries = 2           // デフォルトのリトライ回数
)

// --- グローバル変数とフラグ構造体 ---

// AppFlags はこのアプリケーション固有の永続フラグを保持
type AppFlags struct {
	TimeoutSec int // --timeout タイムアウト
	MaxRetries int // --max-retries リトライ回数
}

var Flags AppFlags // アプリケーション固有フラグにアクセスするためのグローバル変数

// コマンドラインフラグ変数
var (
	feedURL string
)

var rootCmd = &cobra.Command{
	Use:   appName,
	Short: "Webコンテンツ抽出、フィード解析、並列スクレイピングツール",
	Long:  `Webコンテンツの抽出（extract）、RSS/Atomフィードの解析（parse）、および複数のURLの並列抽出（scraper）を実行します。`,
}

// --- 初期化とロジック (clibaseへのコールバックとして利用) ---

// addAppPersistentFlags は、アプリケーション固有の永続フラグをルートコマンドに追加します。
func addAppPersistentFlags(rootCmd *cobra.Command) {
	rootCmd.PersistentFlags().IntVar(
		&Flags.TimeoutSec,
		"timeout",
		defaultTimeoutSec,
		"HTTPリクエストのタイムアウト時間（秒）",
	)
	rootCmd.PersistentFlags().IntVar(
		&Flags.MaxRetries,
		"max-retries",
		defaultMaxRetries,
		"HTTPリクエストのリトライ最大回数",
	)
}

// initAppPreRunE は、clibase共通処理の後に実行される、アプリケーション固有のPersistentPreRunEです。
// NOTE: clibaseの PersistentPreRunE チェーンにより、clibase.Flags.Verbose はこの関数実行前に設定済み
func initAppPreRunE(cmd *cobra.Command, args []string) error {

	timeout := time.Duration(Flags.TimeoutSec) * time.Second

	// clibase.Flags の利用
	if clibase.Flags.Verbose {
		log.Printf("HTTPクライアントのタイムアウトを設定しました (Timeout: %s)。", timeout)
		log.Printf("HTTPクライアントのリトライ回数を設定しました (MaxRetries: %d)。", Flags.MaxRetries)
	}

	return nil
}

// --- エントリポイント ---

// Execute は、rootCmd を実行するメイン関数です。clibaseのExecuteを使用する。
func Execute() {
	// clibase.Execute を使用して、アプリケーションの初期化、フラグ設定、サブコマンドの登録を一括で行う
	clibase.Execute(
		appName,
		addAppPersistentFlags, // カスタムフラグの追加コールバック
		initAppPreRunE,        // カスタムPersistentPreRunEコールバック
		// サブコマンドのリスト (これらは他のファイルで定義されている必要があります)
		extracCommand,
		parseCommand,
		scraperCmd,
	)
}
