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
	// パニックの原因はここ: args.Get(0)がinterface{}(nil)の場合、
	// (*http.Response)への型アサーションが失敗する。
	// そのため、モックの設定側で*http.Response型のnilを返すように修正する。
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

		// 修正: 型付きのnil (*http.Response) を返すように変更
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
