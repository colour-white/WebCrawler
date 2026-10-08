# Go WebCrawler

## Overview
Concurrent Distributed Web Crawler (Go, MongoDB, Docker)

- High-throughput web crawler in Go using worker pools, buffered channels, and dynamic `select` multiplexing to enforce concurrency limits and worker cooldowns.
- Context-aware network fetching with graceful cancellation, HTTP request timeouts, and thread-safe URL deduplication.
- Pipeline-streamed parsed page data into a MongoDB collection with inverted text indexing for fast full-text querying.

## Architecture

The crawler starts from a configured seed URL and creates a shared HTTP client, a URL deduplication set, and a bounded set of workers. A frontier coordinator owns the URL queue and decides which URLs should be crawled next. Before adding a URL to the queue, the coordinator normalizes and validates it, then uses URLSet to avoud visiting the same URL twice.
The coordinator dispatches URLs to wokers through the `jobs` channel, respecting the configured concurrency limit. 

Each worker fetches its assigned page using HTTP request tied to `context.Context`, so cancellation can interrupt network requests. The worker parsed the HTML into a ParsedPage containing the URL, title, text content, and discovered links. Worker send results - including failure signals - back to the coordinator, allowing it to track in-flight jobs correctly.

The coordinator adds newly discovered links to the frontier and forwards successfully parsed pages through the output channel until the page limit is reached or no work remains.

The main goroutine consumes parsed pages and stores them in MongoDB, where the collected content can later be proccessed into an inverted search index.

When crawling finishes or the parent context is cancelled, the coordinator cancels its workers, waits for them to exit, and closes the output channel whithout closing channels that workers may still be using.

```mermaid
flowchart TD
    A["Main: Config + Context"] --> B["Initialize HTTP Client"]
    A --> C["Initialize URLSet"]
    A --> D["Seed URL"]

    D --> E["Frontier Coordinator"]
    C <--> E

    E -->|"URLs via jobs"| F["Worker Pool"]
    F --> G["Fetch HTML using Context"]
    G --> H["Parse HTML"]
    H -->|"ParsedPage or failure signal"| I["Results Channel"]

    I --> E
    E -->|"Validate and deduplicate links"| C
    E --> J["Frontier Queue"]
    J --> E

    E -->|"Parsed pages"| K["Output Channel"]
    K --> L["Main: Store Pages"]
    L --> M["MongoDB"]
    M -.-> N["Future: Inverted Index"]

    A -.->|"Cancellation"| E
    E -.->|"Cancel and wait"| F
```

## Quick start

Note: make sure you have docker installed!

1. Clones the project
    
    `git clone https://github.com/colour-white/WebCrawler`

2. Run MongoDB engine via docker

    `docker-compose up` 

3. Set prefered parameters in `config.json`. For example
    ```json
    {
        "db":{
            "connectionString": "mongodb://admin:password123@localhost:27017/",
            "name":"WebScraper",
            "collectionName":"webpages"
        },
        "startURL": "https://myanimelist.net/",
        "workers": 5,
        "pagesCountToProcess": 100,
        "maxConcurrentRequests": 5,
        "workerCooldown": 1
    }
    ```

4. Run the crawler
    
    `go run .`