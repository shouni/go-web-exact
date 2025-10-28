package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/shouni/go-web-exact/pkg/retry"
)

const (
	// DefaultHTTPTimeout は、HTTPクライアント関連の定数
	DefaultHTTPTimeout = 30 * time.Second
	// MaxResponseBodySize は、あらゆるHTTPレスポンスボディの最大読み込みサイズ
	MaxResponseBodySize = int64(25 * 1024 * 1024) // 25MB
	// UserAgent は、サイトからのブロックを避けるためのUser-Agent
	UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/118.0.0.0 Safari/537.36"
)

// Client はHTTPリクエストと指数バックオフを用いたリトライロジックを管理します。
type Client struct {
	httpClient  HTTPClient
	retryConfig retry.Config
}

// HTTPClient は、*http.Clientと互換性のあるHTTPクライアントのインターフェースを定義します。
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// NonRetryableHTTPError はHTTP 4xx系のステータスコードエラーを示すカスタムエラー型です。
type NonRetryableHTTPError struct {
	StatusCode int
	Body       []byte
}

func (e *NonRetryableHTTPError) Error() string {
	if len(e.Body) > 0 {
		const maxBodyDisplaySize = 1024 // 例: 1KBまで表示
		displayBody := strings.TrimSpace(string(e.Body))
		if len(displayBody) > maxBodyDisplaySize {
			// UTF-8セーフな切り詰め
			runes := []rune(displayBody)
			if len(runes) > maxBodyDisplaySize {
				displayBody = string(runes[:maxBodyDisplaySize]) + "..."
			}
		}
		return fmt.Sprintf("HTTPクライアントエラー (非リトライ対象): ステータスコード %d, ボディ: %s", e.StatusCode, displayBody)
	}
	return fmt.Sprintf("HTTPクライアントエラー (非リトライ対象): ステータスコード %d, ボディなし", e.StatusCode)
}

// Do は HTTPClient インターフェースが持つ Do メソッドを呼び出すラッパーです。
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	return c.httpClient.Do(req)
}

// ClientOption はClientの設定を行うための関数型です。
type ClientOption func(*Client)

// WithHTTPClient はカスタムのHTTPClientを設定します。
func WithHTTPClient(client HTTPClient) ClientOption {
	return func(c *Client) {
		c.httpClient = client
	}
}

// New は新しいClientを初期化します。
// optionsはオプションの設定を受け取ります。
func New(timeout time.Duration, options ...ClientOption) *Client {
	if timeout <= 0 {
		timeout = DefaultHTTPTimeout
	}

	retryCfg := retry.DefaultConfig()
	client := &Client{
		httpClient: &http.Client{
			Timeout: timeout,
		},
		retryConfig: retryCfg,
	}

	for _, opt := range options {
		opt(client)
	}

	return client
}

// WithMaxRetries は最大リトライ回数を設定します。
func (c *Client) WithMaxRetries(max uint64) *Client {
	c.retryConfig.MaxRetries = max
	return c
}

// addCommonHeaders は共通のHTTPヘッダーを設定します。
func (c *Client) addCommonHeaders(req *http.Request) {
	req.Header.Set("User-Agent", UserAgent)
}

// doWithRetry は共通のリトライロジックを実行します。
func (c *Client) doWithRetry(ctx context.Context, operationName string, op func() error) error {
	return retry.Do(
		ctx,
		c.retryConfig,
		operationName,
		op,
		c.isHTTPRetryableError,
	)
}

// FetchDocument はURLからHTMLを取得し、goquery.Documentを返します。
func (c *Client) FetchDocument(url string, ctx context.Context) (*goquery.Document, error) {
	var doc *goquery.Document
	op := func() error {
		var fetchErr error
		doc, fetchErr = c.doFetch(url, ctx)
		return fetchErr // エラーが発生した場合はそのまま返す
	}

	err := c.doWithRetry(
		ctx,
		fmt.Sprintf("URL(%s)のフェッチ", url),
		op,
	)
	if err != nil {
		return nil, err
	}
	return doc, nil
}

// doFetch は実際の一度のHTTP GETリクエストとHTML解析を実行します。
func (c *Client) doFetch(url string, ctx context.Context) (*goquery.Document, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("GETリクエスト作成に失敗しました: %w", err)
	}
	c.addCommonHeaders(req)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("URL %s へのHTTPリクエストに失敗しました (ネットワーク/接続エラー): %w", url, err)
	}

	bodyBytes, err := handleResponse(resp)
	if err != nil {
		return nil, err
	}

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("HTML解析に失敗しました: %w", err)
	}
	return doc, nil
}

// PostJSONAndFetchBytes は指定されたデータをJSONとしてPOSTし、レスポンスボディをバイト配列として返します。
func (c *Client) PostJSONAndFetchBytes(url string, data any, ctx context.Context) ([]byte, error) {
	requestBody, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("JSONデータのシリアライズに失敗しました: %w", err)
	}
	var bodyBytes []byte
	op := func() error {
		var postErr error
		bodyBytes, postErr = c.doPostJSON(url, requestBody, ctx)
		return postErr // エラーが発生した場合はそのまま返す
	}

	err = c.doWithRetry(
		ctx,
		fmt.Sprintf("URL(%s)へのPOSTリクエスト", url),
		op,
	)
	if err != nil {
		return nil, err
	}
	return bodyBytes, nil
}

// doPostJSON は実際の一度のHTTP POSTリクエストを実行し、レスポンスボディを返します。
func (c *Client) doPostJSON(url string, requestBody []byte, ctx context.Context) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(requestBody))
	if err != nil {
		return nil, fmt.Errorf("POSTリクエスト作成に失敗しました: %w", err)
	}
	c.addCommonHeaders(req)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("URL %s へのHTTP POSTリクエストに失敗しました (ネットワーク/接続エラー): %w", url, err)
	}

	return handleResponse(resp)
}

// handleResponse はHTTPレスポンスを処理し、成功した場合はボディをバイト配列として返します。エラーが発生した場合は、ステータスコードに応じてリトライ可能かどうかが判断できるエラーを返します。
func handleResponse(resp *http.Response) ([]byte, error) {
	defer resp.Body.Close()

	if resp.ContentLength > 0 && resp.ContentLength > MaxResponseBodySize {
		return nil, fmt.Errorf("レスポンスボディが最大サイズ (%dバイト) を超えました", MaxResponseBodySize)
	}

	limitedReader := io.LimitReader(resp.Body, MaxResponseBodySize)
	bodyBytes, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, fmt.Errorf("レスポンスボディの読み込みに失敗しました: %w", err)
	}

	// 2xx系は成功
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return bodyBytes, nil
	}

	// 5xx 系: リトライ対象のサーバーエラー
	if resp.StatusCode >= 500 && resp.StatusCode <= 599 {
		return nil, fmt.Errorf("HTTPステータスコードエラー (5xx リトライ対象): %d, 詳細: %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
	}

	// 4xx 系など、その他は非リトライ対象のクライアントエラー
	return nil, &NonRetryableHTTPError{
		StatusCode: resp.StatusCode,
		Body:       bodyBytes,
	}
}

// IsNonRetryableError は与えられたエラーが非リトライ対象のHTTPエラーであるかを判断します。
func IsNonRetryableError(err error) bool {
	if err == nil {
		return false
	}
	var nonRetryable *NonRetryableHTTPError
	return errors.As(err, &nonRetryable)
}

// isHTTPRetryableError はエラーがHTTPリトライ対象かどうかを判定します。
// この関数は retry.ShouldRetryFunc 型のシグネチャを満たします。
func (c *Client) isHTTPRetryableError(err error) bool {
	if err == nil {
		return false
	}
	// 1. Contextエラー（タイムアウト/キャンセル）はリトライ対象
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	// 2. 非リトライ対象エラー（4xx）はリトライしない
	if IsNonRetryableError(err) {
		return false
	}
	// 3. 5xxエラーやネットワークエラー（NonRetryableHTTPErrorでないもの）はすべてリトライ対象
	return true
}
