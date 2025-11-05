package cmd

import (
	"log"
	"os"
	"time"

	clibase "github.com/shouni/go-cli-base"
	"github.com/shouni/go-http-kit/pkg/httpkit"
	"github.com/spf13/cobra"
)

// --- グローバル定数 ---

const (
	appName           = "web-exact" // アプリケーション名を修正
	defaultTimeoutSec = 10          // 秒
	defaultMaxRetries = 5           // デフォルトのリトライ回数

	// 全体処理のタイムアウト定数 (parseCmd, scrapeCmd で利用)
	DefaultOverallTimeout = 20 * time.Second
)

// --- グローバル変数とフラグ構造体 ---

// AppFlags はこのアプリケーション固有の永続フラグを保持
type AppFlags struct {
	TimeoutSec int // --timeout タイムアウト
	MaxRetries int // --max-retries リトライ回数
}

var Flags AppFlags                // アプリケーション固有フラグにアクセスするためのグローバル変数
var globalFetcher httpkit.Fetcher // 全てのサブコマンドで共有されるHTTPクライアント

// 💡 ルートコマンドの定義
var rootCmd = &cobra.Command{
	Use:   appName,
	Short: "Webコンテンツ抽出、フィード解析、並列スクレイピングツール",
	Long:  `Webコンテンツの抽出（extract）、RSS/Atomフィードの解析（parse）、および複数のURLの並列抽出（scrape）を実行します。`,

	// 重要な修正: ルートコマンドは引数を取らないことを明示し、引数エラーを解消
	Args: cobra.NoArgs,
}

// --- 初期化とロジック ---

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
func initAppPreRunE(cmd *cobra.Command, args []string) error {
	// clibase の初期化ロジック (Verboseフラグの処理など) を実行
	// clibase.Execute() を使わないため、Cobraの標準的な方法で初期化処理を呼び出す
	if err := clibase.Init(cmd, args); err != nil {
		return err
	}

	timeout := time.Duration(Flags.TimeoutSec) * time.Second

	if clibase.Flags.Verbose {
		log.Printf("HTTPクライアントのタイムアウトを設定しました (Timeout: %s)。", timeout)
		log.Printf("HTTPクライアントのリトライ回数を設定しました (MaxRetries: %d)。", Flags.MaxRetries)
	}

	// 共有フェッチャーの初期化
	globalFetcher = httpkit.New(timeout, httpkit.WithMaxRetries(Flags.MaxRetries))

	return nil
}

// GetGlobalFetcher は、初期化されたフェッチャーを返す関数 (DIの代わり)
func GetGlobalFetcher() httpkit.Fetcher {
	return globalFetcher
}

// init() 関数でサブコマンドをルートコマンドに追加し、フラグとPreRunEを設定
func init() {
	// 1. サブコマンドの追加
	rootCmd.AddCommand(extractorcmd) // (旧 extractCmd)
	rootCmd.AddCommand(parseCmd)
	rootCmd.AddCommand(scrapeCmd)

	// 2. 永続フラグの設定
	addAppPersistentFlags(rootCmd)

	// 3. PersistentPreRunEの設定 (DIの初期化とclibaseの初期化)
	rootCmd.PersistentPreRunE = initAppPreRunE
}

// --- エントリポイント ---

// Execute は、rootCmd を実行するメイン関数です。
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		// エラーメッセージは Cobra が処理するため、os.Exit(1) のみで十分
		os.Exit(1)
	}
}
