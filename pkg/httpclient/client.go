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
	// HTTPクライアント関連の定数
	DefaultHTTPTimeout = 30 * time.Second
	MaxBodySize        = int64(10 * 1024 * 1024) // 10MB: POSTレスポンスボディの最大読み込みサイズ

	// サイトからのブロックを避けるためのUser-Agent
	UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/118.0.0.0 Safari/537.36"
)

// NonRetryableHTTPError はHTTP 4xx系のステータスコードエラーを示すカスタムエラー型です。
type NonRetryableHTTPError struct {
	StatusCode int
	Body       []byte
}

func (e *NonRetryableHTTPError) Error() string {
	if len(e.Body) > 0 {
		return fmt.Sprintf("HTTPクライアントエラー (非リトライ対象): ステータスコード %d, ボディ: %s", e.StatusCode, strings.TrimSpace(string(e.Body)))
	}
	return fmt.Sprintf("HTTPクライアントエラー (非リトライ対象): ステータスコード %d, ボディなし", e.StatusCode)
}

// Client はHTTPリクエストと指数バックオフを用いたリトライロジックを管理します。
type Client struct {
	httpClient *http.Client
	// ★ 修正: retryMax を削除し、retry.Config を保持
	retryConfig retry.Config
}

// New は、新しいClientを生成します。
func New(timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = DefaultHTTPTimeout
	}

	// ★ 修正: デフォルトのリトライ設定を適用
	retryCfg := retry.DefaultConfig()

	return &Client{
		httpClient: &http.Client{
			Timeout: timeout,
		},
		retryConfig: retryCfg,
	}
}

// WithMaxRetries は最大リトライ回数を設定します。
func (c *Client) WithMaxRetries(max uint64) *Client {
	// ★ 修正: Config に値を設定
	c.retryConfig.MaxRetries = max
	return c
}

// addCommonHeaders は共通のHTTPヘッダーを設定します。
func (c *Client) addCommonHeaders(req *http.Request) {
	req.Header.Set("User-Agent", UserAgent)
}

// FetchDocument はURLからHTMLを取得し、goquery.Documentを返します。
func (c *Client) FetchDocument(url string, ctx context.Context) (*goquery.Document, error) {
	var doc *goquery.Document

	// ★ 修正: op 関数内で backoff.Permanent の処理を削除し、純粋にエラーを返す
	op := func() error {
		var fetchErr error
		doc, fetchErr = c.doFetch(url, ctx)
		return fetchErr // エラーが発生した場合はそのまま返す
	}

	// ★ 修正: c.commonRetryer の代わりに retry.Do() を呼び出す
	err := retry.Do(
		ctx,
		c.retryConfig,
		fmt.Sprintf("URL(%s)のフェッチ", url),
		op,
		c.isHTTPRetryableError, // 新しく定義するHTTP固有の判定関数
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
		return nil, fmt.Errorf("HTTPリクエストに失敗しました (ネットワーク/接続エラー): %w", err)
	}

	defer resp.Body.Close()

	if err := checkResponseForRetry(resp); err != nil {
		return nil, err
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
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

	// ★ 修正: op 関数内で backoff.Permanent の処理を削除し、純粋にエラーを返す
	op := func() error {
		var postErr error
		bodyBytes, postErr = c.doPostJSON(url, requestBody, ctx)
		return postErr // エラーが発生した場合はそのまま返す
	}

	// ★ 修正: c.commonRetryer の代わりに retry.Do() を呼び出す
	err = retry.Do(
		ctx,
		c.retryConfig,
		fmt.Sprintf("URL(%s)へのPOSTリクエスト", url),
		op,
		c.isHTTPRetryableError, // HTTP固有の判定関数
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
		return nil, fmt.Errorf("HTTP POSTリクエストに失敗しました (ネットワーク/接続エラー): %w", err)
	}

	defer resp.Body.Close()

	if err := checkResponseForRetry(resp); err != nil {
		return nil, err
	}

	limitedReader := io.LimitReader(resp.Body, MaxBodySize)
	bodyBytes, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, fmt.Errorf("レスポンスボディの読み込みに失敗しました: %w", err)
	}

	if resp.ContentLength > 0 && resp.ContentLength > MaxBodySize {
		return nil, fmt.Errorf("レスポンスボディが最大サイズ (%dバイト) を超えました", MaxBodySize)
	}

	return bodyBytes, nil
}

// checkResponseForRetry はHTTPレスポンスのステータスコードを評価し、リトライすべきエラーか、非リトライ対象のエラーかを返します。
func checkResponseForRetry(resp *http.Response) error {
	if resp.StatusCode == http.StatusOK {
		return nil
	}

	// 注意: この関数はレスポンスボディを読み込みますが、閉じる責務は持ちません。
	// 呼び出し元が resp.Body.Close() を実行する必要があります。
	limitedReader := io.LimitReader(resp.Body, MaxBodySize)
	bodyBytes, readErr := io.ReadAll(limitedReader)

	if resp.ContentLength > 0 && resp.ContentLength > MaxBodySize {
		return fmt.Errorf("レスポンスボディが最大サイズ (%dバイト) を超えました", MaxBodySize)
	}

	// 5xx 系: リトライ対象のサーバーエラー
	if resp.StatusCode >= 500 && resp.StatusCode <= 599 {
		if readErr != nil {
			return fmt.Errorf("HTTPステータスコードエラー (5xx リトライ対象, ボディ読み込み失敗): %d, 原因: %w", resp.StatusCode, readErr)
		}
		// 5xxエラーをリトライ対象のエラーとしてそのまま返す
		return fmt.Errorf("HTTPステータスコードエラー (5xx リトライ対象): %d, 詳細: %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
	}

	// 4xx 系: 非リトライ対象のクライアントエラー (NonRetryableHTTPError としてラップ)
	if readErr != nil {
		return &NonRetryableHTTPError{
			StatusCode: resp.StatusCode,
		}
	}
	return &NonRetryableHTTPError{
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
	// checkResponseForRetryが5xxの場合に返すエラーはこのカテゴリに入る
	return true
}
