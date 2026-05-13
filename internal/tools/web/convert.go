package web

import (
	"strings"

	md "github.com/JohannesKaufmann/html-to-markdown/v2/converter"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/base"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/commonmark"
)

// HTMLToMarkdown converts article HTML to markdown using base + CommonMark plugins.
func HTMLToMarkdown(html string) (string, error) {
	html = strings.TrimSpace(html)
	if html == "" {
		return "", nil
	}
	conv := md.NewConverter(
		md.WithPlugins(
			base.NewBasePlugin(),
			commonmark.NewCommonmarkPlugin(),
		),
	)
	return conv.ConvertString(html)
}
