package extract_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	// 実際のパッケージのインポートパスに修正してください
	"github.com/shouni/go-web-exact/v2/pkg/extract"
)

// ======================================================================
// モック (Mock) の定義
// ======================================================================

// MockFetcher はテスト用の extract.Fetcher インターフェースの実装です。
type MockFetcher struct {
	htmlContent string
	fetchError  error
}

// FetchBytes はモックされたHTMLをバイト配列として返すか、エラーを返します。
func (m *MockFetcher) FetchBytes(url string, ctx context.Context) ([]byte, error) {
	if m.fetchError != nil {
		return nil, m.fetchError
	}
	// HTMLの内容をそのままバイト配列として返します
	return []byte(m.htmlContent), nil
}

// ======================================================================
// テスト関数
// ======================================================================

func TestNewExtractor(t *testing.T) {
	t.Run("success_with_valid_fetcher", func(t *testing.T) {
		fetcher := &MockFetcher{}
		// NOTE: NewExtractorは extract パッケージの関数を呼び出す
		extractor, err := extract.NewExtractor(fetcher)
		assert.NoError(t, err)
		assert.NotNil(t, extractor)
	})

	t.Run("error_with_nil_fetcher", func(t *testing.T) {
		extractor, err := extract.NewExtractor(nil)
		assert.Error(t, err)
		assert.Nil(t, extractor)
		assert.Contains(t, err.Error(), "Fetcher cannot be nil")
	})
}

// TestFetchAndExtractText は Extractor の主要なメソッドをテストします。
func TestFetchAndExtractText(t *testing.T) {
	// 本体コードの定数と完全に一致させる
	const (
		titlePrefix        = "【記事タイトル】 "
		tableCaptionPrefix = "【表題】 "
	)

	// 本文として抽出されるための十分な長さを持つパラグラフ
	longParagraph := "This is a long paragraph with more than twenty characters and it should be extracted as body content."

	testCases := []struct {
		name              string
		html              string
		url               string
		fetchErr          error
		expectedText      string
		expectedBodyFound bool
		expectedError     bool
	}{
		// 1. ネットワークエラーのテスト
		{
			name:          "fetch_error",
			fetchErr:      errors.New("network timeout"),
			expectedError: true,
		},

		// 2. タイトルのみのドキュメントのテスト (短いテキストは無視される)
		{
			name:              "document_with_title_only",
			html:              `<html><head><title>Test Title</title></head><body><p>Short text</p></body></html>`,
			expectedText:      titlePrefix + "Test Title",
			expectedBodyFound: false, // 短い段落は本文と見なされない
			expectedError:     false,
		},

		// 3. メインコンテンツとタイトルのドキュメントのテスト (長い段落を抽出)
		{
			name:              "document_with_main_content_and_title",
			html:              fmt.Sprintf(`<html><head><title>Title</title></head><body><main><p>%s</p></main></body></html>`, longParagraph),
			expectedText:      titlePrefix + "Title" + "\n\n" + longParagraph,
			expectedBodyFound: true,
			expectedError:     false,
		},

		// 4. 見出しと段落のドキュメントのテスト (H1/H2の整形と\n\n区切り)
		{
			name: "document_with_headings_and_paragraphs",
			html: fmt.Sprintf(`<html><head><title>Test Page</title></head><body><article>
                <h1>Heading 1 Long Enough Title</h1>
                <p>Short</p>
                <h2>H2 Long Enough</h2>
                <p>%s</p>
               </article></body></html>`, longParagraph),
			expectedText: titlePrefix + "Test Page" + "\n\n" +
				"## Heading 1 Long Enough Title" + "\n\n" +
				"## H2 Long Enough" + "\n\n" +
				longParagraph,
			expectedBodyFound: true,
			expectedError:     false,
		},

		// 5. テーブルと pre タグのテスト (順序と短い段落の無視を反映)
		{
			name: "document_with_table_and_pre",
			url:  "http://example.com/table-and-pre",
			html: `<html><head><title>Code Table</title></head><body><main>
                   <article>
                      <p>Intro text</p>
                      <table><caption>Data Table</caption><tr><td>Col1</td><td>Val1</td></tr></table>
                      <pre>func hello() {}</pre>
                   </article>
                   </main></body></html>`,
			// 修正: "Intro text" (10文字) は MinParagraphLength=20 より短いため無視される
			expectedText:      "【記事タイトル】 Code Table\n\n【表題】 Data Table\nCol1 | Val1\n\n```\nfunc hello() {}\n```",
			expectedBodyFound: true,
			expectedError:     false,
		},

		// 6. リストアイテムのテスト (短いテキストでも抽出される)
		{
			name: "document_with_list_items",
			html: `<html><head><title>List Test</title></head><body><main><ul><li>Item 1</li><li>Item 2</li></ul></main></body></html>`,
			expectedText: titlePrefix + "List Test" + "\n\n" +
				"Item 1" + "\n\n" +
				"Item 2",
			expectedBodyFound: true,
			expectedError:     false,
		},

		// 7. エラーケース: 何も抽出できない場合
		{
			name:              "empty_document_error",
			html:              `<html><head><title></title></head><body></body></html>`,
			expectedText:      "",
			expectedBodyFound: false,
			expectedError:     true, // "webページから何も抽出できませんでした" が期待される
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// モックのセットアップ
			fetcher := &MockFetcher{
				htmlContent: tc.html,
				fetchError:  tc.fetchErr,
			}

			// Extractorの初期化 (extract.Fetcher インターフェースとして渡す)
			extractor, err := extract.NewExtractor(fetcher)
			assert.NoError(t, err)

			ctx := context.Background()

			// 実行
			actualText, actualBodyFound, err := extractor.FetchAndExtractText("https://example.com/"+tc.name, ctx)

			// 1. エラーチェック
			if tc.expectedError {
				assert.Error(t, err, "エラーが期待されていましたが、エラーがありませんでした")
				return
			}
			assert.NoError(t, err, "予期せぬエラーが発生しました")

			// 2. 本文抽出フラグチェック
			assert.Equal(t, tc.expectedBodyFound, actualBodyFound, "hasBodyFoundが期待値と異なります")

			// 3. 抽出テキストチェック
			assert.Equal(t, tc.expectedText, actualText, "抽出されたテキストが期待値と異なります")
		})
	}
}
