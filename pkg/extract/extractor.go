package extract

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/PuerkitoBio/goquery"
	textUtils "github.com/shouni/go-utils/text"
)

// Extractor は、Fetcher を使ってコンテンツ抽出プロセスを管理します。
type Extractor struct {
	fetcher Fetcher
}

// NewExtractor は、新しいExtractorのインスタンスを生成します。
func NewExtractor(fetcher Fetcher) (*Extractor, error) {
	if fetcher == nil {
		return nil, fmt.Errorf("extract.NewExtractor: Fetcher cannot be nil")
	}
	return &Extractor{
		fetcher: fetcher,
	}, nil
}

// ----------------------------------------------------------------------
// 定数定義 (解析関連のみ)
// ----------------------------------------------------------------------
const (
	MinParagraphLength   = 20
	MinHeadingLength     = 3
	mainContentSelectors = "article, main, div[role='main'], #main, #content, .post-content, .article-body, .entry-content, .markdown-body, .readme"
	noiseSelectors       = ".related-posts, .social-share, .comments, .ad-banner, .advertisement"

	// textExtractionTags は本文抽出に使用するHTMLタグを定義します。
	textExtractionTags = "p, h1, h2, h3, h4, h5, h6, li, blockquote"

	titlePrefix        = "【記事タイトル】 "
	tableCaptionPrefix = "【表題】 "
)

// ----------------------------------------------------------------------
// メイン関数 (メソッド化)
// ----------------------------------------------------------------------

// FetchAndExtractText は指定されたURLからコンテンツを取得し、整形されたテキストを抽出します。
func (e *Extractor) FetchAndExtractText(url string, ctx context.Context) (text string, hasBodyFound bool, err error) {
	// 1. Fetcherから生のバイト配列を取得 (通信の責務)
	htmlBytes, err := e.fetcher.FetchBytes(url, ctx)
	if err != nil {
		return "", false, err
	}

	// 2. Extractor内でgoquery.Documentに変換 (解析の責務)
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(htmlBytes))
	if err != nil {
		return "", false, fmt.Errorf("HTML解析に失敗しました: %w", err)
	}

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

	// 4. すべての関連コンテンツ要素（p, h*, li, blockquote, table, pre）を結合したセレクター
	//    このセレクターは、goqueryによってDOMの出現順に走査されます。
	//    親要素が子の要素を含む場合（例：<div><p>...</p><table>...</table></div>）、
	//    goqueryは重複を排除し、DOMの深さ優先探索順序で要素を返します。
	contentSelectors := textExtractionTags + ", table, pre"

	mainContent.Find(contentSelectors).Each(func(i int, s *goquery.Selection) {
		var content string

		if s.Is("table") {
			// テーブルの処理
			content = processTable(s)
		} else if s.Is("pre") {
			// pre タグ (コードブロック) の処理
			preText := strings.TrimSpace(s.Text())
			if preText != "" {
				content = "```\n" + preText + "\n```"
			}
		} else {
			// 一般的なテキスト要素 (p, h*, li, blockquote) の処理
			content = e.processGeneralElement(s)
		}

		if content != "" {
			parts = append(parts, content)
		}
	})

	// 5. 抽出結果の検証
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
	tempSelection := s.Clone()
	tempSelection.Find("pre, table").Remove() // 子孫の pre, table を除去

	text := tempSelection.Text()
	text = textUtils.NormalizeText(text)

	isHeading := s.Is("h1, h2, h3, h4, h5, h6")
	isListItem := s.Is("li")
	if text == "" {
		return ""
	}

	// ... (既存の長さチェックロジックを続行) ...
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
func processTable(s *goquery.Selection) string { // パッケージレベル関数に
	var tableContent []string
	captionText := strings.TrimSpace(s.Find("caption").First().Text())
	if captionText != "" {
		tableContent = append(tableContent, tableCaptionPrefix+captionText)
	}
	s.Find("tr").Each(func(rowIndex int, row *goquery.Selection) {
		var rowTexts []string
		row.Find("th, td").Each(func(cellIndex int, cell *goquery.Selection) {
			rowTexts = append(rowTexts, textUtils.NormalizeText(cell.Text()))
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
