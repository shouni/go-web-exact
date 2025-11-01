package extract

import (
	"context"
)

// ----------------------------------------------------------------------
// 依存性の定義 (DIP)
// ----------------------------------------------------------------------

// Fetcher は、HTMLドキュメントの生バイト配列を取得する機能のインターフェースを定義します。
// Extractor は、この抽象に依存します。
type Fetcher interface {
	FetchBytes(url string, ctx context.Context) ([]byte, error)
}
