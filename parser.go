package main

import (
	"io"
	"strings"

	"golang.org/x/net/html"
)

func parsePage(pageURL string, body io.Reader) (ParsedPage, error) {
	page := ParsedPage{
		URL: pageURL,
	}

	z := html.NewTokenizer(body)

	var content strings.Builder

	skipDepth := 0
	inTitle := false

	for {
		tt := z.Next()

		switch tt {
		case html.ErrorToken:
			if z.Err() == io.EOF {
				page.Content = strings.TrimSpace(content.String())
				page.Title = strings.TrimSpace(page.Title)
				return page, nil
			}

			return page, z.Err()

		case html.StartTagToken:
			token := z.Token()

			switch token.Data {
			case "script", "style", "noscript", "template":
				skipDepth++

			case "title":
				inTitle = true

			case "a":
				for _, attr := range token.Attr {
					if attr.Key != "href" {
						continue
					}

					resolvedURL, err := resolveURL(page.URL, attr.Val)
					if err != nil {
						ErrorLog.Printf(
							"Error resolving URL %q: %v",
							attr.Val,
							err,
						)
						continue
					}

					page.Links = append(page.Links, resolvedURL)
				}
			}

		case html.EndTagToken:
			token := z.Token()

			switch token.Data {
			case "title":
				inTitle = false

			case "script", "style", "noscript", "template":
				if skipDepth > 0 {
					skipDepth--
				}
			}

		case html.TextToken:
			text := strings.TrimSpace(string(z.Raw()))

			if text == "" {
				continue
			}

			if inTitle {
				if page.Title != "" {
					page.Title += " "
				}

				page.Title += text
				continue
			}

			if skipDepth == 0 {
				if content.Len() > 0 {
					content.WriteByte(' ')
				}

				content.WriteString(text)
			}
		}
	}
}
