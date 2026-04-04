package scraper

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

// mockExtractor はテスト用の Extractor 実装なのだ。
type mockExtractor struct {
	fetchFunc func(ctx context.Context, url string) (string, bool, error)
	callCount int32
}

func (m *mockExtractor) FetchAndExtractText(ctx context.Context, url string) (string, bool, error) {
	atomic.AddInt32(&m.callCount, 1)
	return m.fetchFunc(ctx, url)
}

func TestConcurrent_Run(t *testing.T) {
	t.Run("正常系: すべてのURLからコンテンツが抽出できること", func(t *testing.T) {
		mock := &mockExtractor{
			fetchFunc: func(ctx context.Context, url string) (string, bool, error) {
				return "content for " + url, true, nil
			},
		}

		// NewParallelScraper -> New に変更
		s := New(mock, WithMaxConcurrency(2))
		urls := []string{"http://example.com/1", "http://example.com/2"}

		// ScrapeInParallel -> Run に変更
		results := s.Run(context.Background(), urls)

		if len(results) != 2 {
			t.Errorf("期待する結果件数は 2 ですが、%d 件でした", len(results))
		}
		if atomic.LoadInt32(&mock.callCount) != 2 {
			t.Errorf("Extractorの呼び出し回数が不正です: %d", mock.callCount)
		}

		for _, res := range results {
			if res.Error != nil {
				t.Errorf("URL %s で予期せぬエラーが発生しました: %v", res.URL, res.Error)
			}
		}
	})

	t.Run("異常系: Extractorがエラーを返す場合に結果に含まれること", func(t *testing.T) {
		mock := &mockExtractor{
			fetchFunc: func(ctx context.Context, url string) (string, bool, error) {
				return "", false, errors.New("network error")
			},
		}

		s := New(mock)
		urls := []string{"http://err.com"}

		results := s.Run(context.Background(), urls)

		if len(results) != 1 || results[0].Error == nil {
			t.Fatal("エラーが正しく結果に格納されていません")
		}
	})

	t.Run("異常系: 本文が見つからない場合にエラーとして処理されること", func(t *testing.T) {
		mock := &mockExtractor{
			fetchFunc: func(ctx context.Context, url string) (string, bool, error) {
				return "", false, nil // hasBodyFound = false
			},
		}

		s := New(mock)
		urls := []string{"http://nobody.com"}

		results := s.Run(context.Background(), urls)

		if results[0].Error == nil {
			t.Error("本文未検出時のエラーが生成されていません")
		}
	})

	t.Run("レートリミットの検証: 短時間で大量のリクエストを送った際に時間がかかること", func(t *testing.T) {
		mock := &mockExtractor{
			fetchFunc: func(ctx context.Context, url string) (string, bool, error) {
				return "ok", true, nil
			},
		}

		// 100ms 間隔で設定
		interval := 100 * time.Millisecond
		s := New(mock, WithRateLimit(interval))
		urls := []string{"u1", "u2", "u3"}

		start := time.Now()
		_ = s.Run(context.Background(), urls)
		duration := time.Since(start)

		// 1つ目は即時、2つ目は100ms後、3つ目は200ms後
		// 合計で最低 200ms 以上の経過が必要なのだ。
		expectedMin := 200 * time.Millisecond
		if duration < expectedMin {
			t.Errorf("レートリミットが機能していない可能性があります。所要時間: %v, 期待値: > %v", duration, expectedMin)
		}
	})
}
