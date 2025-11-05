package cmd

import (
	"log"
	"time"

	clibase "github.com/shouni/go-cli-base"
	"github.com/shouni/go-http-kit/pkg/httpkit"
	"github.com/shouni/go-web-exact/v2/pkg/extract"
	"github.com/spf13/cobra"
)

// --- グローバル定数 ---

const (
	appName           = "web-exact" // アプリケーション名を修正
	defaultTimeoutSec = 10          // 秒
	defaultMaxRetries = 5           // デフォルトのリトライ回数

	// 全体処理のタイムアウト定数 (parseCmd, scraperCmd で利用)
	DefaultOverallTimeout = 20 * time.Second
)

// --- グローバル変数とフラグ構造体 ---

// AppFlags はこのアプリケーション固有の永続フラグを保持
type AppFlags struct {
	TimeoutSec int // --timeout タイムアウト
	MaxRetries int // --max-retries リトライ回数
}

var Flags AppFlags                // アプリケーション固有フラグにアクセスするためのグローバル変数
var globalFetcher extract.Fetcher // または feed.Fetcher (両方満たすため)

// 💡 ルートコマンドの定義 (clibaseがルートコマンドを生成するため、UseとLongのみ残し、他は削除)
var rootCmd = &cobra.Command{
	Use:   appName,
	Short: "Webコンテンツ抽出、フィード解析、並列スクレイピングツール",
	Long:  `Webコンテンツの抽出（extract）、RSS/Atomフィードの解析（parse）、および複数のURLの並列抽出（scraper）を実行します。`,
	// Args, PersistentPreRunE, init() のロジックは clibase に任せる
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

	// 共有フェッチャーの初期化
	globalFetcher = httpkit.New(
		timeout,
		httpkit.WithMaxRetries(uint64(Flags.MaxRetries)),
	)

	return nil
}

// GetGlobalFetcher は、初期化されたフェッチャーを返す関数 (DIの代わり)
func GetGlobalFetcher() httpkit.Fetcher {
	return globalFetcher
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
		extractorcmd,
		parseCmd,
		scraperCmd,
	)
	// clibase.Execute() の中で os.Exit(1) が処理されるため、ここでは不要
}

// 💡 注意: clibaseの新しい設計では、init() 関数は不要になりました。
// 以前の init() 関数の内容は Execute() 関数に移譲されています。
