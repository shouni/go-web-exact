package client

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/shouni/go-utils" // utils パッケージをインポート
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockHTTPClient は http.Client の Do メソッドをモックします。
// Doer インターフェースを満たします。
type MockHTTPClient struct {
	mock.Mock
}

func (m *MockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	args := m.Called(req)
	// エラーは常に args.Error(1) から取得
	err := args.Error(1)

	// レスポンスが存在する場合のみ型アサーションを行う
	if args.Get(0) != nil {
		// MockHTTPClient は Doer を満たし、内部の client.Client の httpClient フィールドは Doer 型
		return args.Get(0).(*http.Response), err
	}
	return nil, err
}

func TestNew(t *testing.T) {
	t.Run("default timeout", func(t *testing.T) {
		client := New(0)
		assert.Equal(t, DefaultHTTPTimeout, client.httpClient.(*http.Client).Timeout)
	})
	t.Run("custom timeout", func(t *testing.T) {
		timeout := 30 * time.Second
		client := New(timeout)
		assert.Equal(t, timeout, client.httpClient.(*http.Client).Timeout)
	})
	t.Run("with HTTP client option", func(t *testing.T) {
		mockClient := new(MockHTTPClient)
		client := New(10*time.Second, WithHTTPClient(mockClient))
		assert.Equal(t, mockClient, client.httpClient) // httpClient は Doer 型
	})
}

// WithMaxRetries は ClientOption なので New 関数内でテストする
func TestWithMaxRetries(t *testing.T) {
	t.Run("sets max retries via option", func(t *testing.T) {
		client := New(0, WithMaxRetries(5))
		// 内部の utils.Config の値を確認
		assert.Equal(t, uint64(5), client.retryConfig.MaxRetries)
	})
}

func TestNonRetryableHTTPError_Error(t *testing.T) {
	tests := []struct {
		name       string
		body       []byte
		expected   string
		statusCode int
	}{
		{"non-empty body", []byte("error body"), "HTTPクライアントエラー (非リトライ対象): ステータスコード 400, ボディ: error body", 400},
		{"empty body", nil, "HTTPクライアントエラー (非リトライ対象): ステータスコード 400, ボディなし", 400},
		{"truncated body", []byte(strings.Repeat("a", 1025)), "HTTPクライアントエラー (非リトライ対象): ステータスコード 400, ボディ: " + strings.Repeat("a", 1024) + "...", 400},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &NonRetryableHTTPError{StatusCode: tt.statusCode, Body: tt.body}
			assert.Equal(t, tt.expected, err.Error())
		})
	}
}

func TestFetchBytes(t *testing.T) {
	url := "https://example.com"
	ctx := context.Background()

	t.Run("successful fetch", func(t *testing.T) {
		mockClient := new(MockHTTPClient)
		expectedBody := []byte("<html></html>")
		mockBody := bytes.NewReader(expectedBody)
		mockResponse := &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(mockBody),
		}
		mockClient.On("Do", mock.Anything).Return(mockResponse, nil).Once()

		client := New(0, WithHTTPClient(mockClient))
		body, err := client.FetchBytes(url, ctx)
		assert.NoError(t, err)
		assert.Equal(t, expectedBody, body)
		mockClient.AssertExpectations(t)
	})

	t.Run("http client error", func(t *testing.T) {
		mockClient := new(MockHTTPClient)
		var resp *http.Response
		// Mockが呼び出されることを確認するが、リトライは無効化する
		mockClient.On("Do", mock.Anything).Return(resp, errors.New("network error"))

		// リトライが発動しないように、WithMaxRetries(0) を設定したClientを生成
		client := New(0,
			WithHTTPClient(mockClient),
			WithMaxRetries(0),
		)

		body, err := client.FetchBytes(url, ctx)
		assert.Error(t, err)
		assert.Nil(t, body)

		mockClient.AssertExpectations(t)
		// 呼び出し回数が1回であることを明示的に検証
		mockClient.AssertNumberOfCalls(t, "Do", 1)
	})

	t.Run("non-retryable error", func(t *testing.T) {
		mockClient := new(MockHTTPClient)
		mockBody := bytes.NewReader([]byte("bad request"))
		mockResponse := &http.Response{
			StatusCode: http.StatusBadRequest,
			Body:       io.NopCloser(mockBody),
		}
		mockClient.On("Do", mock.Anything).Return(mockResponse, nil).Once()

		client := New(0, WithHTTPClient(mockClient))
		body, err := client.FetchBytes(url, ctx)
		assert.Error(t, err)
		assert.Nil(t, body)
		mockClient.AssertExpectations(t)
	})
}

// --- リトライロジックの検証テスト ---
func TestFetchBytes_WithRetries(t *testing.T) {
	url := "https://example.com"
	ctx := context.Background()
	// 修正: retry.Config を utils.Config に変更
	retryCfg := utils.Config{
		MaxRetries: 2, // 初回含め最大3回実行
	}

	t.Run("successful fetch after retries (network error)", func(t *testing.T) {
		mockClient := new(MockHTTPClient)
		expectedBody := []byte("Success")
		var resp *http.Response // 型付きのnil

		// 1回目: ネットワークエラー (リトライ対象)
		mockClient.On("Do", mock.Anything).Return(
			resp, errors.New("temporary network error"),
		).Once()
		// 2回目: サーバーエラー (リトライ対象)
		mockClient.On("Do", mock.Anything).Return(
			&http.Response{StatusCode: http.StatusGatewayTimeout, Body: io.NopCloser(bytes.NewReader(nil))}, nil,
		).Once()
		// 3回目: 成功 (リトライ終了)
		mockClient.On("Do", mock.Anything).Return(
			&http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(expectedBody))}, nil,
		).Once()

		client := &Client{
			httpClient:  mockClient,
			retryConfig: retryCfg,
		}
		body, err := client.FetchBytes(url, ctx)
		assert.NoError(t, err)
		assert.Equal(t, expectedBody, body)
		mockClient.AssertExpectations(t)
		mockClient.AssertNumberOfCalls(t, "Do", 3) // 3回呼ばれたことを確認
	})

	t.Run("failure after all retries exhausted", func(t *testing.T) {
		mockClient := new(MockHTTPClient)
		var resp *http.Response

		// MaxRetries=2 のため、Doは合計3回（初回＋2回リトライ）呼ばれる
		// 全てエラーを返す
		mockClient.On("Do", mock.Anything).Return(resp, errors.New("network error")).Times(3)

		client := &Client{
			httpClient:  mockClient,
			retryConfig: retryCfg,
		}
		body, err := client.FetchBytes(url, ctx)
		assert.Error(t, err)
		assert.Nil(t, body)

		// 3回呼ばれたことを確認
		mockClient.AssertNumberOfCalls(t, "Do", 3)
	})

	t.Run("non-retryable error stops immediately", func(t *testing.T) {
		mockClient := new(MockHTTPClient)
		mockBody := bytes.NewReader([]byte("Client error"))

		// 1回目: 404 Not Found (非リトライ対象)
		mockClient.On("Do", mock.Anything).Return(
			&http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(mockBody)}, nil,
		).Once()

		client := &Client{
			httpClient:  mockClient,
			retryConfig: retryCfg, // リトライ設定があっても発動しない
		}
		body, err := client.FetchBytes(url, ctx)
		assert.Error(t, err)
		assert.True(t, IsNonRetryableError(err))
		assert.Nil(t, body)

		// 1回しか呼ばれていないことを確認
		mockClient.AssertNumberOfCalls(t, "Do", 1)
	})
}

func TestPostJSONAndFetchBytes(t *testing.T) {
	url := "https://example.com"
	data := map[string]string{"key": "test"}
	ctx := context.Background()

	t.Run("successful post and fetch", func(t *testing.T) {
		mockClient := new(MockHTTPClient)
		expectedBody := []byte(`{"key":"value"}`)
		mockBody := bytes.NewReader(expectedBody)
		mockResponse := &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(mockBody),
		}
		mockClient.On("Do", mock.Anything).Return(mockResponse, nil).Once()

		client := New(0, WithHTTPClient(mockClient))
		body, err := client.PostJSONAndFetchBytes(url, data, ctx)
		assert.NoError(t, err)
		assert.Equal(t, expectedBody, body)
		mockClient.AssertExpectations(t)
	})

	t.Run("json serialization error", func(t *testing.T) {
		client := &Client{}
		// marshalできないチャネルを渡す
		body, err := client.PostJSONAndFetchBytes(url, make(chan int), ctx)
		assert.Error(t, err)
		assert.Nil(t, body)
	})

	t.Run("non-retryable error", func(t *testing.T) {
		mockClient := new(MockHTTPClient)
		mockBody := bytes.NewReader([]byte("bad request"))
		mockResponse := &http.Response{
			StatusCode: http.StatusBadRequest,
			Body:       io.NopCloser(mockBody),
		}
		mockClient.On("Do", mock.Anything).Return(mockResponse, nil).Once()

		client := New(0, WithHTTPClient(mockClient))
		body, err := client.PostJSONAndFetchBytes(url, data, ctx)
		assert.Error(t, err)
		assert.Nil(t, body)
		mockClient.AssertExpectations(t)
	})
}

// PostJSONAndFetchBytes に対するリトライテスト
func TestPostJSONAndFetchBytes_WithRetries(t *testing.T) {
	url := "https://example.com"
	data := map[string]string{"key": "test"}
	ctx := context.Background()

	// 修正: retry.Config を utils.Config に変更
	retryCfg := utils.Config{
		MaxRetries: 1, // 初回含め最大2回実行
	}

	t.Run("successful post after 5xx retry", func(t *testing.T) {
		mockClient := new(MockHTTPClient)
		expectedBody := []byte(`{"status":"ok"}`)
		mockResponse := &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader(expectedBody)),
		}

		// 1回目: サーバーエラー (503 Service Unavailable)
		mockClient.On("Do", mock.Anything).Return(
			&http.Response{StatusCode: http.StatusServiceUnavailable, Body: io.NopCloser(bytes.NewReader(nil))}, nil,
		).Once()
		// 2回目: 成功
		mockClient.On("Do", mock.Anything).Return(
			mockResponse, nil,
		).Once()

		client := &Client{
			httpClient:  mockClient,
			retryConfig: retryCfg,
		}
		body, err := client.PostJSONAndFetchBytes(url, data, ctx)

		assert.NoError(t, err)
		assert.Equal(t, expectedBody, body)
		mockClient.AssertNumberOfCalls(t, "Do", 2)
	})
}

func TestIsNonRetryableError(t *testing.T) {
	t.Run("nil error", func(t *testing.T) {
		assert.False(t, IsNonRetryableError(nil))
	})
	t.Run("non-retryable error", func(t *testing.T) {
		err := &NonRetryableHTTPError{}
		assert.True(t, IsNonRetryableError(err))
	})
	t.Run("other error type", func(t *testing.T) {
		assert.False(t, IsNonRetryableError(errors.New("some error")))
	})
}
