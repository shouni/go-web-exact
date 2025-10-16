package web

import (
	"context"
	"fmt"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// ----------------------------------------------------------------------
// 依存性の定義 (DI)
// ----------------------------------------------------------------------

// Fetcher は、HTMLドキュメントを取得する機能のインターフェースを定義します。
type Fetcher interface {
	FetchDocument(url string, ctx context.Context) (*goquery.Document, error)
}

// Extractor は、Fetcher を使ってコンテンツ抽出プロセスを管理します。
type Extractor struct {
	fetcher Fetcher
}

// NewExtractor は、新しいExtractorのインスタンスを生成します。
func NewExtractor(fetcher Fetcher) *Extractor {
	return &Extractor{
		fetcher: fetcher,
	}
}

// ----------------------------------------------------------------------
// 定数定義 (解析関連のみ)
// ----------------------------------------------------------------------
const (
	MinParagraphLength   = 20
	MinHeadingLength     = 3
	mainContentSelectors = "article, main, div[role='main'], #main, #content, .post-content, .article-body, .entry-content, .markdown-body, .readme"
	noiseSelectors       = ".related-posts, .social-share, .comments, .ad-banner, .advertisement"

	// pre を textExtractionTags から除去 (以前の修正)
	textExtractionTags = "p, h1, h2, h3, h4, h5, h6, li, blockquote"

	titlePrefix        = "【記事タイトル】 "
	tableCaptionPrefix = "【表題】 "
)

// ----------------------------------------------------------------------
// ヘルパー関数
// ----------------------------------------------------------------------

func normalizeText(text string) string {
	text = strings.ReplaceAll(text, "\n", " ")
	text = strings.ReplaceAll(text, "\t", " ")
	text = strings.Join(strings.Fields(text), " ")
	return strings.TrimSpace(text)
}

// ----------------------------------------------------------------------
// メイン関数 (メソッド化)
// ----------------------------------------------------------------------

// FetchAndExtractText は指定されたURLからコンテンツを取得し、整形されたテキストを抽出します。
func (e *Extractor) FetchAndExtractText(url string, ctx context.Context) (text string, hasBodyFound bool, err error) {
	doc, err := e.fetcher.FetchDocument(url, ctx)
	if err != nil {
		return "", false, err
	}

	// ★ 修正: メソッド呼び出しに変更
	return e.extractContentText(doc)
}

// extractContentText はgoquery.Documentから本文とタイトルを抽出し、整形します。
func (e *Extractor) extractContentText(doc *goquery.Document) (text string, hasBodyFound bool, err error) {
	var parts []string
	// 1. ページタイトルを抽出
	pageTitle := strings.TrimSpace(doc.Find("title").First().Text())
	if pageTitle != "" {
		parts = append(parts, titlePrefix+pageTitle)
	}
	// 2. メインコンテンツの特定
	mainContent := e.findMainContent(doc)
	// 3. ノイズ要素の除去
	mainContent.Find(noiseSelectors).Remove()

	// 4. テーブル、pre 以外のテキスト要素を取得し、テキストを結合
	mainContent.Find(textExtractionTags).Each(func(i int, s *goquery.Selection) {
		if content := e.processGeneralElement(s); content != "" {
			parts = append(parts, content)
		}
	})

	// 5. pre タグを個別に処理
	mainContent.Find("pre").Each(func(i int, s *goquery.Selection) {
		preText := strings.TrimSpace(s.Text())
		if preText != "" {
			parts = append(parts, "```\n"+preText+"\n```")
		}
	})

	// 6. テーブルを個別に処理
	mainContent.Find("table").Each(func(i int, s *goquery.Selection) {
		// ★ 修正: メソッド呼び出しに変更
		if content := e.processTable(s); content != "" {
			parts = append(parts, content)
		}
	})
	// 7. 抽出結果の検証
	return e.validateAndFormatResult(parts)
}

// findMainContent はメインコンテントを取得
func (e *Extractor) findMainContent(doc *goquery.Document) *goquery.Selection {
	mainContent := doc.Find(mainContentSelectors).First()
	if mainContent.Length() == 0 {
		mainContent = doc.Selection.
			Not("header, footer, nav, aside, .sidebar, script, style, form")
	}
	return mainContent
}

// processGeneralElement は生成する
func (e *Extractor) processGeneralElement(s *goquery.Selection) string {
	text := s.Text()
	text = normalizeText(text)
	isHeading := s.Is("h1, h2, h3, h4, h5, h6")
	isListItem := s.Is("li")
	if text == "" {
		return ""
	}
	if isHeading {
		if len(text) > MinHeadingLength {
			return "## " + text
		}
	} else {
		if isListItem || len(text) > MinParagraphLength {
			return text
		}
	}
	return ""
}

// processTable は goquery.Selection からテーブルの内容を抽出し、整形します。
// Extractor の状態に依存しないため、パッケージレベル関数とすることも可能ですが、
// Extractor のロジックの一部として一貫性のためメソッドとして保持します。
func (e *Extractor) processTable(s *goquery.Selection) string {
	var tableContent []string
	captionText := strings.TrimSpace(s.Find("caption").First().Text())
	if captionText != "" {
		tableContent = append(tableContent, tableCaptionPrefix+captionText)
	}
	s.Find("tr").Each(func(rowIndex int, row *goquery.Selection) {
		var rowTexts []string
		row.Find("th, td").Each(func(cellIndex int, cell *goquery.Selection) {
			rowTexts = append(rowTexts, normalizeText(cell.Text()))
		})
		tableContent = append(tableContent, strings.Join(rowTexts, " | "))
	})
	if len(tableContent) > 0 {
		return strings.Join(tableContent, "\n")
	}
	return ""
}

// validateAndFormatResult はフォーマットを確認
func (e *Extractor) validateAndFormatResult(parts []string) (text string, hasBodyFound bool, err error) {
	if len(parts) == 0 {
		return "", false, fmt.Errorf("webページから何も抽出できませんでした")
	}
	isTitleOnly := len(parts) == 1 && strings.HasPrefix(parts[0], titlePrefix)
	if isTitleOnly {
		return parts[0], false, nil
	}
	return strings.Join(parts, "\n\n"), true, nil
}
