package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"
)

func fetchPage(ctx context.Context, pageURL string, client *http.Client) (io.ReadCloser, error) {

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "MySearchCrawler/0.1")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()

		return nil, fmt.Errorf("failed to retrieve page: %s", resp.Status)
	}

	return resp.Body, nil
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

func processURL(ctx context.Context,
	jobs <-chan string, client *http.Client, results chan<- *ParsedPage, wg *sync.WaitGroup) {

	defer wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case pageURL, ok := <-jobs:
			if !ok {
				return
			}
			InfoLog.Printf("Visiting: %s\n", pageURL)

			body, err := fetchPage(ctx, pageURL, client)

			if err != nil {
				if ctx.Err() != nil {
					return
				}
				ErrorLog.Printf("Error fetching %s: %v", pageURL, err)
			}

			parsedPage, err := parsePage(pageURL, body)

			body.Close()

			if err != nil {
				ErrorLog.Printf("Error parsing %s: %v", pageURL, err)
				continue
			}

			select {
			case results <- &parsedPage:
			case <-ctx.Done():
				return
			}

			timer := time.NewTimer(time.Second * time.Duration(Config.WorkerCooldown))

			select {
			case <-timer.C:
			case <-ctx.Done():
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				return
			}

		}
	}

}
func fetchData(ctx context.Context, startURL string) <-chan *ParsedPage {
	client := &http.Client{Timeout: 15 * time.Second}
	jobs := make(chan string)
	urlSet := NewURLSet()
	urlSet.Add(startURL)

	results := make(chan *ParsedPage, Config.Workers)

	var wg sync.WaitGroup
	for range Config.Workers {
		wg.Add(1)
		go processURL(ctx, jobs, client, results, &wg)
	}

	output := make(chan *ParsedPage, Config.PagesCountToProcess)

	go func() {
		defer close(output)

		crawlCtx, cancel := context.WithCancel(ctx)

		defer cancel()

		queue := []string{startURL}

		inFlight := 0
		visitedCount := 0

		for visitedCount < Config.PagesCountToProcess {

			if len(queue) == 0 && inFlight == 0 {
				break
			}

			var jobChan chan string
			var nextURL string

			if len(queue) > 0 && inFlight < Config.MaxConcurrentRequests {
				jobChan = jobs
				nextURL = queue[0]
			}

			select {
			case jobChan <- nextURL:
				queue = queue[1:]
				inFlight++

			case page := <-results:
				inFlight--
				if page == nil {
					continue
				}
				visitedCount++
				select {
				case output <- page:
				case <-crawlCtx.Done():
					return
				}

				for _, link := range page.Links {
					if !validURL(link) {
						continue
					}
					if urlSet.Add(link) {
						queue = append(queue, link)
					}
				}
			}

		}

		cancel()
		close(jobs)
		wg.Wait()

	}()

	return output
}

var Config config

func main() {
	loadConfig("config.json")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dbClient, err := connectToMongoDB(
		Config.DB.ConnectionString,
	)

	if err != nil {
		ErrorLog.Printf(
			"Error connecting to mongo db: %s",
			err,
		)
		return
	}

	db, err := createIndex(
		dbClient,
		Config.DB.Name,
		ctx,
	)

	if err != nil {
		ErrorLog.Printf(
			"Error creating collection: %s",
			err,
		)
		return
	}

	data := fetchData(ctx, Config.StartURL)

	for page := range data {
		if err := insertParsedPage(db, ctx, page); err != nil {
			WarningLog.Printf(
				"Error inserting data into webpages: %s",
				err,
			)
		}
	}
}
