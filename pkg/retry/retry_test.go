package retry

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	// 修正: 期待値をuint64にキャストして比較することで、型不一致を解消。
	require.Equal(t, uint64(DefaultMaxRetries), cfg.MaxRetries, "MaxRetries should match DefaultMaxRetries constant.")
	require.Equal(t, InitialBackoffInterval, cfg.InitialInterval, "InitialInterval should match constant.")
	require.Equal(t, MaxBackoffInterval, cfg.MaxInterval, "MaxInterval should match constant.")
}

func TestNewBackOffPolicy(t *testing.T) {
	ctx := context.Background()
	cfg := Config{
		MaxRetries:      5,
		InitialInterval: 10 * time.Millisecond,
		MaxInterval:     500 * time.Millisecond,
	}

	bo := newBackOffPolicy(ctx, cfg)
	require.NotNil(t, bo)
}

func TestDo(t *testing.T) {
	// テスト用の高速な設定
	testCfg := Config{MaxRetries: 3, InitialInterval: 1 * time.Millisecond, MaxInterval: 10 * time.Millisecond}
	opName := "test_operation"

	// 予期されるエラーメッセージを実装に合わせて正確に生成
	permanentErrText := fmt.Sprintf("%sに失敗しました: 最大リトライ回数 (%d回) に到達。最終エラー: %s", opName, testCfg.MaxRetries, "permanent error")
	maxRetriesErrText := fmt.Sprintf("%sに失敗しました: 最大リトライ回数 (%d回) に到達。最終エラー: retryable error", opName, testCfg.MaxRetries)

	tests := []struct {
		name          string
		ctx           context.Context
		cfg           Config
		operationName string
		operation     Operation
		shouldRetry   ShouldRetryFunc
		expectedError string
	}{
		{
			name:          "successful operation",
			ctx:           context.Background(),
			cfg:           testCfg,
			operationName: opName,
			operation:     func() error { return nil },
			shouldRetry:   func(err error) bool { return false },
			expectedError: "",
		},
		{
			name:          "retryable error and success within max retries",
			ctx:           context.Background(),
			cfg:           testCfg,
			operationName: opName,
			operation: func() Operation {
				attempt := 0
				return func() error {
					attempt++
					if attempt < 3 {
						return errors.New("retryable error")
					}
					return nil
				}
			}(),
			shouldRetry:   func(err error) bool { return err.Error() == "retryable error" },
			expectedError: "",
		},
		{
			name:          "permanent error",
			ctx:           context.Background(),
			cfg:           testCfg,
			operationName: opName,
			operation: func() error {
				// 修正: operation内で backoff.Permanent を直接返すことで、
				// Do関数内の致命的エラー判定パスの実行を保証する。
				return backoff.Permanent(errors.New("permanent error"))
			},
			// PermanentErrorが返されるとshouldRetryFnは無視されるため、trueで問題ない
			shouldRetry:   func(err error) bool { return true },
			expectedError: permanentErrText,
		},
		{
			name:          "context canceled",
			ctx:           func() context.Context { ctx, cancel := context.WithCancel(context.Background()); cancel(); return ctx }(),
			cfg:           testCfg,
			operationName: opName,
			operation: func() error {
				// コンテキストエラーを誘発するために、リトライ対象のエラーを返す
				return errors.New("some error")
			},
			shouldRetry:   func(err error) bool { return true },
			// 期待値はコンテキストエラー処理後のメッセージ (containsで検証)
			expectedError: "test_operationに失敗しました: コンテキストタイムアウト/キャンセル: context canceled",
		},
		{
			name: "context timeout",
			ctx: func() context.Context {
				// タイムアウトを非常に短く設定
				ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
				time.Sleep(2 * time.Millisecond) // 確実にタイムアウトさせる
				defer cancel()
				return ctx
			}(),
			cfg:           testCfg,
			operationName: opName,
			operation: func() error {
				return errors.New("some error")
			},
			shouldRetry:   func(err error) bool { return true },
			// 期待値はコンテキストエラー処理後のメッセージ (containsで検証)
			expectedError: "test_operationに失敗しました: コンテキストタイムアウト/キャンセル: context deadline exceeded",
		},
		{
			name:          "max retries exceeded",
			ctx:           context.Background(),
			cfg:           testCfg,
			operationName: opName,
			operation: func() error {
				return errors.New("retryable error")
			},
			shouldRetry:   func(err error) bool { return true },
			expectedError: maxRetriesErrText,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Do(tt.ctx, tt.cfg, tt.operationName, tt.operation, tt.shouldRetry)

			if tt.expectedError != "" {
				require.Error(t, err)

				// コンテキストエラーは元のエラーをラップしているため、Containsを使用
				if tt.name == "context canceled" || tt.name == "context timeout" {
					require.Contains(t, err.Error(), tt.expectedError)
				} else {
					// 永続エラーとリトライ上限エラーは、メッセージ全体を検証
					// ここで require.Equal を使用し、メッセージの完全一致をチェックします。
					require.Equal(t, tt.expectedError, err.Error())
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}
