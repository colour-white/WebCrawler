# Go WebScrawler

A simple webscrawler written in golang.

Uses `MaxConcurrentRequests`workers to collect data from `PageCountToVisit`pages. Then uploads data into mongodb collection with inverted index for fast text lookups.
