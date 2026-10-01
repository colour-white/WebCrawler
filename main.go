package main

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"golang.org/x/net/html"
)

const PageCountToVisit = 10

type URLSet struct {
	visited map[string]struct{}
	mu      sync.Mutex
}

func NewURLSet() *URLSet {
	return &URLSet{
		visited: make(map[string]struct{}),
	}
}
func (s *URLSet) Add(url string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.visited[url]; exists {
		return false
	}
	s.visited[url] = struct{}{}
	return true
}

type ParsedPage struct {
	Url   string
	Links []string
	Text  strings.Builder
}

func parsePage(url string, body io.Reader) (ParsedPage, error) {

	page := ParsedPage{Url: url}

	z := html.NewTokenizer(body)
	skipDepth := 0
	for {

		tt := z.Next()

		switch tt {
		case html.ErrorToken:
			if z.Err() == io.EOF {
				return page, nil
			}
			return page, z.Err()

		case html.StartTagToken:
			token := z.Token()
			switch token.Data {
			case "script", "style", "noscript", "template":
				skipDepth++
			case "a":
				for _, attr := range token.Attr {
					if attr.Key == "href" {
						resolvedURL, err := resolveURL(page.Url, attr.Val)
						if err != nil {
							fmt.Printf("Error resolving URL: %v\n", err)
							continue
						}
						page.Links = append(page.Links, resolvedURL)
					}
				}
			}
		case html.EndTagToken:
			token := z.Token()
			switch token.Data {
			case "script", "style", "noscript", "template":
				if skipDepth > 0 {
					skipDepth--
				}
			}
		case html.TextToken:

			if skipDepth == 0 {
				text := strings.TrimSpace(string(z.Raw()))
				if text != "" {
					page.Text.WriteString(text)
					page.Text.WriteString(" ")
				}
			}
		}
	}

}

func fetchPage(url string, client *http.Client) (io.Reader, error) {

	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to retrieve page: %s", response.Status)
	}

	return response.Body, nil

}

func resolveURL(baseURL, relativeURL string) (string, error) {

	base, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}

	target, err := url.Parse(relativeURL)
	if err != nil {
		return "", err
	}
	return base.ResolveReference(target).String(), nil
}

func main() {

	stack := make([]string, 0)
	urlSet := NewURLSet()

	// url stack -> fetchPage -> parsePage -> extract links -> url stack

	stack = append(stack, "https://books.toscrape.com/")

	client := &http.Client{}

	for visitedPages := 0; len(stack) > 0 && visitedPages < PageCountToVisit; visitedPages++ {
		page, err := fetchPage(stack[0], client)
		if err != nil {
			fmt.Printf("Error fetching page: %v\n", err)
			return
		}
		parsedPage, err := parsePage(stack[0], page)
		if err != nil {
			fmt.Printf("Error parsing page: %v\n", err)
		} else {
			fmt.Println(parsedPage.Links)
			fmt.Println(parsedPage.Text.String())

		}
		stack = stack[1:]

		for _, link := range parsedPage.Links {
			if urlSet.Add(link) {
				stack = append(stack, link)
			}
		}

		stack = append(stack, parsedPage.Links...)
	}
}
