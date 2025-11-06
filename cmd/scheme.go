package cmd

import (
	"fmt"
	"net/url"
)

// ensureScheme は、URLのスキームが存在しない場合に https:// を補完します。
func ensureScheme(feedURL string) (string, error) {
	// 1. まず現在のURLをパース
	parsedURL, err := url.Parse(feedURL)
	if err != nil {
		return "", fmt.Errorf("URLのパースエラー: %w", err)
	}

	// 2. スキームが既に存在する場合のチェック
	if parsedURL.Scheme != "" {
		if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
			return "", fmt.Errorf("無効なURLスキームです。httpまたはhttpsを指定してください: %s", feedURL)
		}
		// 既存のスキームを尊重
		return feedURL, nil
	}

	// 3. スキームがない場合、HTTPSをデフォルトとして付与
	return "https://" + feedURL, nil
}
