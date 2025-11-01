package client

import (
	"context"
	"net/http"
	"time"

	"github.com/shouni/go-http-kit/pkg/httpkit"
)

// ----------------------------------------------------------------------
// 定数とインターフェース
// ----------------------------------------------------------------------

const (
	// DefaultHTTPTimeout は、デフォルトのHTTPタイムアウトです。
	DefaultHTTPTimeout = 10 * time.Second
)

// Doer は、標準の *http.Client.Do()と互換性のあるHTTPクライアントのインターフェースを定義します。
type Doer interface {
	Do(req *http.Request) (*http.Response, error)
}

// Client は httpkit.Client をラップし、リトライロジックをカプセル化します。
// httpkit.Client を埋め込むことで、Doer, Fetcher, Poster などのインターフェースを自動的に満たします。
type Client struct {
	*httpkit.Client // httpkit.Client を埋め込み、そのすべてのメソッドを継承
}

// ----------------------------------------------------------------------
// 設定とコンストラクタ
// ----------------------------------------------------------------------

// ClientOption はClientの設定を行うための関数型です。
// 内部の httpkit.Client のオプションを適用するためのラッパーです。
type ClientOption func(*Client)

// WithHTTPClient はカスタムのDoerを設定します。
// 内部の httpkit.Client にカスタムDoerを設定します。
func WithHTTPClient(doer Doer) ClientOption {
	return func(c *Client) {
		// 埋め込み型の httpkit.Client のオプションを呼び出す
		httpkit.WithHTTPClient(doer)(c.Client)
	}
}

// WithMaxRetries は最大リトライ回数を設定します。
// 内部の httpkit.Client のリトライ設定を更新します。
func WithMaxRetries(max uint64) ClientOption {
	return func(c *Client) {
		httpkit.WithMaxRetries(max)(c.Client)
	}
}

// New は新しいClientを初期化します。
// 内部で httpkit.New を呼び出し、設定オプションを適用します。
func New(timeout time.Duration, options ...ClientOption) *Client {
	// 1. httpkit.Client を初期化
	// timeout は内部の http.Client の Timeout に使用されます。
	kitClient := httpkit.New(timeout)

	c := &Client{
		Client: kitClient,
	}

	// 2. ClientOption を c.Client (つまり httpkit.Client) に適用
	for _, opt := range options {
		opt(c)
	}

	return c
}

// ----------------------------------------------------------------------
// httpkit メソッドの利用
// ----------------------------------------------------------------------

// FetchBytes は URL からコンテンツをフェッチし、生のバイト配列として返します。
// リトライロジックは httpkit.Client が処理します。
func (c *Client) FetchBytes(url string, ctx context.Context) ([]byte, error) {
	return c.Client.FetchBytes(url, ctx)
}

// PostJSONAndFetchBytes は指定されたデータをJSONとしてPOSTし、レスポンスボディをバイト配列として返します。
// リトライロジックは httpkit.Client が処理します。
func (c *Client) PostJSONAndFetchBytes(url string, data any, ctx context.Context) ([]byte, error) {
	return c.Client.PostJSONAndFetchBytes(url, data, ctx)
}

// IsNonRetryableError は与えられたエラーが非リトライ対象のHTTPエラーであるかを判断します。
// httpkit の同名関数を呼び出します。
func IsNonRetryableError(err error) bool {
	return httpkit.IsNonRetryableError(err)
}

// HandleLimitedResponse は、指定されたレスポンスボディを、最大サイズに制限して読み込みます。
// httpkit の同名関数を呼び出します。
func HandleLimitedResponse(resp *http.Response, limit int64) ([]byte, error) {
	return httpkit.HandleLimitedResponse(resp, limit)
}
