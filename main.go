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

const PageCountToVisit = 100
const MaxConcurrentRequests = 100

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

func fetchPage(url string, client *http.Client) (io.ReadCloser, error) {

	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}

	if response.StatusCode != http.StatusOK {
		response.Body.Close()
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
func validURL(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}

	return u.Scheme == "http" || u.Scheme == "https"
}
func main() {
	client := &http.Client{}
	jobs := make(chan string)
	results := make(chan []string)

	startURL := "https://books.toscrape.com/"

	urlSet := NewURLSet()
	urlSet.Add(startURL)

	for range MaxConcurrentRequests {

		go func() {

			for pageURL := range jobs {

				fmt.Printf("Visiting: %s\n", pageURL)

				page, err := fetchPage(pageURL, client)
				if err != nil {
					fmt.Printf("Error fetching page: %v\n", err)
					results <- nil
					continue
				}

				parsedPage, err := parsePage(pageURL, page)
				page.Close()
				if err != nil {
					fmt.Printf("Error parsing page: %v\n", err)
					results <- nil
					continue
				}

				// fmt.Println(parsedPage.Links)
				// fmt.Println(parsedPage.Text.String())

				var newLinks []string

				for _, link := range parsedPage.Links {
					if !validURL(link) {
						continue
					}

					if urlSet.Add(link) {
						newLinks = append(newLinks, link)
					}
				}

				results <- newLinks
			}

		}()

	}

	queue := []string{startURL}

	inFlight := 0
	visitedCount := 0

	for visitedCount < PageCountToVisit {

		var jobsChan chan string
		var nextURL string

		if len(queue) > 0 && inFlight < MaxConcurrentRequests {
			jobsChan = jobs
			nextURL = queue[0]
		}

		select {
		case jobsChan <- nextURL:
			queue = queue[1:]
			inFlight++
		case newLinks := <-results:
			inFlight--
			visitedCount++
			queue = append(queue, newLinks...)
		}
		if visitedCount >= PageCountToVisit {
			break
		}
	}
	close(jobs)

	fmt.Printf("Visited %d pages\n", visitedCount)
}
