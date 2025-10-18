package retry

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/cenkalti/backoff/v4"
)

const (
	// リトライ関連の定数
	DefaultMaxRetries = 3 // 最大リトライ回数

	// バックオフのカスタム設定
	InitialBackoffInterval = 500 * time.Millisecond
	MaxBackoffInterval     = 5 * time.Second
)

// Operation はリトライ可能な処理を表す関数です。成功時は nil を返します。
type Operation func() error

// ShouldRetryFunc はエラーを受け取り、そのエラーがリトライ可能かどうかを判定する関数です。
type ShouldRetryFunc func(error) bool

// Config はリトライ動作を設定するための構造体です。
type Config struct {
	MaxRetries      uint64
	InitialInterval time.Duration
	MaxInterval     time.Duration
}

// DefaultConfig は推奨されるデフォルト設定を返します。
func DefaultConfig() Config {
	return Config{
		MaxRetries:      DefaultMaxRetries,
		InitialInterval: InitialBackoffInterval,
		MaxInterval:     MaxBackoffInterval,
	}
}

// newBackOffPolicy は設定とコンテキストから backoff.BackOff を生成します。
func newBackOffPolicy(ctx context.Context, cfg Config) backoff.BackOff {
	b := backoff.NewExponentialBackOff()
	b.InitialInterval = cfg.InitialInterval
	b.MaxInterval = cfg.MaxInterval

	bo := backoff.WithMaxRetries(b, cfg.MaxRetries)
	return backoff.WithContext(bo, ctx)
}

// Do は指数バックオフとカスタムエラー判定を使用して操作をリトライします。
// Configを引数で受け取ることで、特定のクライアント構造体（Client）への依存を排除しています。
func Do(ctx context.Context, cfg Config, operationName string, op Operation, shouldRetryFn ShouldRetryFunc) error {
	bo := newBackOffPolicy(ctx, cfg)

	retryableOp := func() error {
		err := op()
		if err == nil {
			return nil // 成功
		}
		if shouldRetryFn(err) {
			return err // リトライ対象
		}
		// 永続エラーとしてラップし、即時終了
		return backoff.Permanent(err)
	}

	err := backoff.Retry(retryableOp, bo)
	if err != nil {
		// 永続的エラーかどうかを最初に判定する
		if pErr, ok := err.(*backoff.PermanentError); ok {
			return fmt.Errorf("%sに失敗しました: 致命的なエラーのためリトライを中止: %w", operationName, pErr.Err)
		}
		// コンテキストのキャンセルまたはタイムアウト
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return fmt.Errorf("%sに失敗しました: コンテキストタイムアウト/キャンセル: %w", operationName, err)
		}
		// その他のエラーはリトライ上限到達とみなす
		return fmt.Errorf("%sに失敗しました: 最大リトライ回数 (%d回) に到達。最終エラー: %w", operationName, cfg.MaxRetries, err)
	}
	return nil
}
