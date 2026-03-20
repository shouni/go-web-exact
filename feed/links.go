package feed

import (
	"github.com/mmcdole/gofeed"
)

// LinkSource は、リンクアイテムのリストを提供できる任意の型を表します。
type LinkSource interface {
	GetLinks() []string
}

// FeedAdapter は gofeed.Feed を LinkSource に適合させるためのアダプターです。
type FeedAdapter struct {
	*gofeed.Feed
}

// NewFeedAdapter は gofeed.Feed から新しいアダプターを作成します。
func NewFeedAdapter(feed *gofeed.Feed) *FeedAdapter {
	return &FeedAdapter{Feed: feed}
}

// GetLinks は LinkSource インターフェースを満たし、gofeed.Feed からリンクを抽出します。
func (a *FeedAdapter) GetLinks() []string {
	if a.Feed == nil || len(a.Items) == 0 {
		return []string{}
	}

	urls := make([]string, 0, len(a.Items))
	for _, item := range a.Items {
		if item.Link != "" {
			urls = append(urls, item.Link)
		}
	}
	return urls
}

// GetTitlesMap はフィードアイテムからURLをキー、タイトルを値とするマップを返します。
func (a *FeedAdapter) GetTitlesMap() map[string]string {
	titlesMap := make(map[string]string)

	if a.Feed == nil || len(a.Items) == 0 {
		return titlesMap
	}

	for _, item := range a.Items {
		if item.Link != "" && item.Title != "" {
			titlesMap[item.Link] = item.Title
		}
	}
	return titlesMap
}

// 汎用的な抽出関数 (オプション)

// GetAllLinks は LinkSource インターフェースを満たすオブジェクトからリンクを抽出する汎用関数です。
func GetAllLinks(source LinkSource) []string {
	if source == nil {
		return []string{}
	}
	return source.GetLinks()
}
