package pipeline

import (
	"context"
	"fmt"
	"time"

	"github.com/shouni/go-http-kit/pkg/httpkit"
	"github.com/shouni/go-web-exact/v2/pkg/extract"
)

// ExtractURLContent は、URLからコンテンツを取得し、整形されたテキストを返すメインの処理パイプラインです。
// text: 抽出された整形済みテキスト、hasBody: 本文が見つかったかどうか、err: エラー
func ExtractURLContent(rawURL string) (text string, hasBody bool, err error) { // 💡 修正 2: 戻り値の型を修正
	const (
		clientTimeout  = 30 * time.Second
		overallTimeout = 60 * time.Second
	)

	// 1. 外部の Fetcher 実装を初期化 (依存性の初期化)
	fetcher := httpkit.New(clientTimeout)

	// 2. Extractor を初期化 (DI)
	extractor, err := extract.NewExtractor(fetcher)
	if err != nil {
		return "", false, fmt.Errorf("Extractorの初期化エラー: %w", err)
	}

	// 3. 全体処理のコンテキストを設定
	ctx, cancel := context.WithTimeout(context.Background(), overallTimeout)
	defer cancel()

	// 4. 抽出の実行
	text, hasBody, err = extractor.FetchAndExtractText(rawURL, ctx)
	if err != nil {
		return "", false, fmt.Errorf("コンテンツ抽出エラー: %w", err)
	}

	return text, hasBody, nil
}
