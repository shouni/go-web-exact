package httpclient

import (
	"bytes"
	"context"
	"errors"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/shouni/go-web-exact/pkg/retry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockHTTPClient struct {
	mock.Mock
}

func (m *MockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	args := m.Called(req)
	return args.Get(0).(*http.Response), args.Error(1)
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
		assert.Equal(t, mockClient, client.httpClient)
	})
}

func TestWithMaxRetries(t *testing.T) {
	client := &Client{
		retryConfig: retry.Config{},
	}
	client.WithMaxRetries(5)
	assert.Equal(t, uint64(5), client.retryConfig.MaxRetries)
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

func TestFetchDocument(t *testing.T) {
	t.Run("successful fetch", func(t *testing.T) {
		mockClient := new(MockHTTPClient)
		mockBody := bytes.NewReader([]byte("<html></html>"))
		mockResponse := &http.Response{
			StatusCode: http.StatusOK,
			Body:       ioutil.NopCloser(mockBody),
		}
		mockClient.On("Do", mock.Anything).Return(mockResponse, nil)

		client := &Client{httpClient: mockClient}
		doc, err := client.FetchDocument("https://example.com", context.Background())
		assert.NoError(t, err)
		assert.NotNil(t, doc)
		mockClient.AssertExpectations(t)
	})
	t.Run("http client error", func(t *testing.T) {
		mockClient := new(MockHTTPClient)

		var resp *http.Response
		mockClient.On("Do", mock.Anything).Return(resp, errors.New("network error"))

		client := &Client{httpClient: mockClient}
		doc, err := client.FetchDocument("https://example.com", context.Background())
		assert.Error(t, err)
		assert.Nil(t, doc)
		mockClient.AssertExpectations(t)
	})
	t.Run("non-retryable error", func(t *testing.T) {
		mockClient := new(MockHTTPClient)
		mockBody := bytes.NewReader([]byte("bad request"))
		mockResponse := &http.Response{
			StatusCode: http.StatusBadRequest,
			Body:       ioutil.NopCloser(mockBody),
		}
		mockClient.On("Do", mock.Anything).Return(mockResponse, nil)

		client := &Client{httpClient: mockClient}
		doc, err := client.FetchDocument("https://example.com", context.Background())
		assert.Error(t, err)
		assert.Nil(t, doc)
		mockClient.AssertExpectations(t)
	})
}

// --- リトライロジックの検証テスト ---
func TestFetchDocument_WithRetries(t *testing.T) {
	// MinDelay/MaxDelay を削除し、デフォルト設定を使用
	retryCfg := retry.Config{
		MaxRetries: 2,
	}

	t.Run("successful fetch after retries (network error)", func(t *testing.T) {
		mockClient := new(MockHTTPClient)
		mockBody := bytes.NewReader([]byte("<html></html>"))
		var resp *http.Response // 型付きのnil

		// 1回目: ネットワークエラー (リトライ対象)
		mockClient.On("Do", mock.Anything).Return(
			resp, errors.New("temporary network error"),
		).Once()
		// 2回目: サーバーエラー (リトライ対象)
		mockClient.On("Do", mock.Anything).Return(
			&http.Response{StatusCode: http.StatusGatewayTimeout, Body: ioutil.NopCloser(bytes.NewReader(nil))}, nil,
		).Once()
		// 3回目: 成功 (リトライ終了)
		mockClient.On("Do", mock.Anything).Return(
			&http.Response{StatusCode: http.StatusOK, Body: ioutil.NopCloser(mockBody)}, nil,
		).Once()

		client := &Client{
			httpClient:  mockClient,
			retryConfig: retryCfg,
		}
		doc, err := client.FetchDocument("https://example.com", context.Background())
		assert.NoError(t, err)
		assert.NotNil(t, doc)
		mockClient.AssertExpectations(t)
		mockClient.AssertNumberOfCalls(t, "Do", 3) // 3回呼ばれたことを確認
	})

	t.Run("failure after all retries exhausted", func(t *testing.T) {
		mockClient := new(MockHTTPClient)
		var resp *http.Response

		// MaxRetries=2 のため、Doは合計3回（初回＋2回リトライ）呼ばれる
		// 1回目: エラー
		mockClient.On("Do", mock.Anything).Return(resp, errors.New("network error 1")).Once()
		// 2回目: エラー
		mockClient.On("Do", mock.Anything).Return(resp, errors.New("network error 2")).Once()
		// 3回目: エラー (MaxRetries に到達)
		mockClient.On("Do", mock.Anything).Return(resp, errors.New("final network error")).Once()

		client := &Client{
			httpClient:  mockClient,
			retryConfig: retryCfg,
		}
		doc, err := client.FetchDocument("https://example.com", context.Background())
		assert.Error(t, err)
		assert.Nil(t, doc)

		// 3回呼ばれたことを確認
		mockClient.AssertNumberOfCalls(t, "Do", 3)
	})

	t.Run("non-retryable error stops immediately", func(t *testing.T) {
		mockClient := new(MockHTTPClient)
		mockBody := bytes.NewReader([]byte("Client error"))

		// 1回目: 404 Not Found (非リトライ対象)
		mockClient.On("Do", mock.Anything).Return(
			&http.Response{StatusCode: http.StatusNotFound, Body: ioutil.NopCloser(mockBody)}, nil,
		).Once()

		client := &Client{
			httpClient:  mockClient,
			retryConfig: retryCfg, // リトライ設定があっても発動しない
		}
		doc, err := client.FetchDocument("https://example.com", context.Background())
		assert.Error(t, err)
		assert.True(t, IsNonRetryableError(err))
		assert.Nil(t, doc)

		// 1回しか呼ばれていないことを確認
		mockClient.AssertNumberOfCalls(t, "Do", 1)
	})
}

func TestPostJSONAndFetchBytes(t *testing.T) {
	t.Run("successful post and fetch", func(t *testing.T) {
		mockClient := new(MockHTTPClient)
		mockBody := bytes.NewReader([]byte(`{"key":"value"}`))
		mockResponse := &http.Response{
			StatusCode: http.StatusOK,
			Body:       ioutil.NopCloser(mockBody),
		}
		mockClient.On("Do", mock.Anything).Return(mockResponse, nil)

		client := &Client{httpClient: mockClient}
		data := map[string]string{"key": "test"}
		body, err := client.PostJSONAndFetchBytes("https://example.com", data, context.Background())
		assert.NoError(t, err)
		assert.Equal(t, []byte(`{"key":"value"}`), body)
		mockClient.AssertExpectations(t)
	})
	t.Run("json serialization error", func(t *testing.T) {
		client := &Client{}
		body, err := client.PostJSONAndFetchBytes("https://example.com", make(chan int), context.Background())
		assert.Error(t, err)
		assert.Nil(t, body)
	})
	t.Run("non-retryable error", func(t *testing.T) {
		mockClient := new(MockHTTPClient)
		mockBody := bytes.NewReader([]byte("bad request"))
		mockResponse := &http.Response{
			StatusCode: http.StatusBadRequest,
			Body:       ioutil.NopCloser(mockBody),
		}
		mockClient.On("Do", mock.Anything).Return(mockResponse, nil)

		client := &Client{httpClient: mockClient}
		data := map[string]string{"key": "test"}
		body, err := client.PostJSONAndFetchBytes("https://example.com", data, context.Background())
		assert.Error(t, err)
		assert.Nil(t, body)
		mockClient.AssertExpectations(t)
	})
}

// PostJSONAndFetchBytes に対するリトライテスト
func TestPostJSONAndFetchBytes_WithRetries(t *testing.T) {
	// MinDelay/MaxDelay を削除し、デフォルト設定を使用
	retryCfg := retry.Config{
		MaxRetries: 1, // 初回含め最大2回実行
	}

	t.Run("successful post after 5xx retry", func(t *testing.T) {
		mockClient := new(MockHTTPClient)
		mockResponse := &http.Response{
			StatusCode: http.StatusOK,
			Body:       ioutil.NopCloser(bytes.NewReader([]byte(`{"status":"ok"}`))),
		}

		// 1回目: サーバーエラー (503 Service Unavailable)
		mockClient.On("Do", mock.Anything).Return(
			&http.Response{StatusCode: http.StatusServiceUnavailable, Body: ioutil.NopCloser(bytes.NewReader(nil))}, nil,
		).Once()
		// 2回目: 成功
		mockClient.On("Do", mock.Anything).Return(
			mockResponse, nil,
		).Once()

		client := &Client{
			httpClient:  mockClient,
			retryConfig: retryCfg,
		}
		data := map[string]string{"key": "test"}
		body, err := client.PostJSONAndFetchBytes("https://example.com", data, context.Background())

		assert.NoError(t, err)
		assert.Equal(t, []byte(`{"status":"ok"}`), body)
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
