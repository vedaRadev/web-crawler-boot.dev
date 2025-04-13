package main

import (
	"os"
	"fmt"
	"io"
	"net/http"
	neturl "net/url"
	nethtml "golang.org/x/net/html"
	"strings"
	"sync"
	semaphore "golang.org/x/sync/semaphore"
	"context"
	"strconv"
	"slices"
	"time"
)

func normalizeURL(url string) (string, error) {
	parsed, err := neturl.Parse(url)
	if err != nil { return "", err }
	return strings.TrimRight(parsed.Host + parsed.Path, "/"), nil
}

func getURLsFromHTML(htmlBody string, rawBaseURL string) ([]string, error) {
	var result []string

	baseURL, err := neturl.Parse(rawBaseURL)
	if err != nil {
		fmt.Printf("failed to parse base url %v\n", rawBaseURL)
		return result, err
	}

	htmlReader := strings.NewReader(htmlBody)
	htmlRoot, err := nethtml.Parse(htmlReader)
	if err != nil { return result, err }

	visitStack := make([]*nethtml.Node, 1, 1)
	visitStack[0] = htmlRoot
	for len(visitStack) > 0 {
		node := visitStack[len(visitStack) - 1]
		visitStack = visitStack[:len(visitStack) - 1]

		// Visit this node
		if node.Type == nethtml.ElementNode && node.Data == "a" {
			for _, attr := range node.Attr {
				if attr.Key == "href" && len(attr.Val) > 0 {
					href, err := neturl.Parse(attr.Val)
					if err != nil { continue }
					resolvedURL := baseURL.ResolveReference(href)
					result = append(result, resolvedURL.String())
				}
			}
		}

		// Prep to visit children
		for child := node.LastChild; child != nil; child = child.PrevSibling {
			visitStack = append(visitStack, child)
		}
	}

	return result, nil
}

func getHTML(rawURL string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 1500 * time.Millisecond)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", rawURL, nil)
	if err != nil { return "", err }

	resp, err := http.DefaultClient.Do(req)
	if err != nil { return "", err }
	if resp.StatusCode >= 400 && resp.StatusCode <= 499 {
		return "", fmt.Errorf("400 status: %v", resp.StatusCode)
	}
	contentType := resp.Header.Get("content-type")
	if strings.Contains("txt/html", contentType) {
		return "", fmt.Errorf("invalid content type: %v", contentType)
	}

	html, err := io.ReadAll(resp.Body)
	if err != nil { return "", err }

	return string(html), nil
}

// TODO instead of a sempahore use a buffered channel for concurrency control
func crawlPage(
	base *string,
	urlToCrawl string,
	urlsFound map[string]int,
	numPagesCrawled *int,
	maxPages int,
	mu *sync.Mutex,
	sem *semaphore.Weighted,
	sem_ctx context.Context,
	wg *sync.WaitGroup,
) {
	// this should've been incremented BEFORE entering the function
	defer wg.Done()

	err := sem.Acquire(sem_ctx, 1)
	if err != nil {
		fmt.Printf("%v - failed to acquire semaphore\n", urlToCrawl)
		return
	}
	defer sem.Release(1)

	fmt.Println()
	fmt.Printf("visiting: %v\n", urlToCrawl)
	normalizedUrl, err := normalizeURL(urlToCrawl)
	if err != nil {
		fmt.Printf("failed to normalize %v: %v\n", urlToCrawl, err)
		return
	}
	fmt.Printf("normalized: %v\n", normalizedUrl)

	mu.Lock()
	if _, exists := urlsFound[normalizedUrl]; exists {
		urlsFound[normalizedUrl] += 1
		mu.Unlock()
		return // no need to fetch and crawl again
	} 

	urlsFound[normalizedUrl] = 1
	if *numPagesCrawled < maxPages {
		*numPagesCrawled += 1
	} else {
		mu.Unlock()
		fmt.Printf("%v - page crawl threshold exceeded (%v/%v), aborting\n", urlToCrawl, *numPagesCrawled, maxPages)
		return
	}
	mu.Unlock()

	fmt.Printf("%v: fetching html\n", urlToCrawl)
	html, err := getHTML(urlToCrawl)
	if err != nil {
		fmt.Printf("%v: failed to fetch HTML - %v\n", urlToCrawl, err)
		return
	}

	fmt.Printf("%v: starting crawl\n", urlToCrawl)
	discovered, err := getURLsFromHTML(html, urlToCrawl)
	if err != nil {
		fmt.Printf("failed to crawl %v: %v\n", urlToCrawl, err)
		return
	}

	fmt.Printf("%v: crawl done, found %v urls\n", urlToCrawl, len(discovered))
	for i, discoveredUrl := range discovered {
		// Limit crawl to a single domain
		if strings.Contains(discoveredUrl, *base) {
			wg.Add(1)
			fmt.Printf("\t%v: %v - %v SPAWN\n", i, urlToCrawl, discoveredUrl)
			go crawlPage(base, discoveredUrl, urlsFound, numPagesCrawled, maxPages, mu, sem, sem_ctx, wg)
		} else {
			fmt.Printf("\t%v: %v - %v DISCARD\n", i, urlToCrawl, discoveredUrl)
		}
	}

	fmt.Printf("%v - EXIT\n", urlToCrawl)
}

func main() {
	args := os.Args[1:]
	if len(args) != 3 {
		fmt.Println("usage: ./crawler URL MAX_CONCURRENCY MAX_PAGES")
		os.Exit(1)
	}

	baseURL := args[0]
	maxConcurrency, err := strconv.Atoi(args[1])
	if err != nil {
		fmt.Println("MAX_CONCURRENCY must be an integer")
		os.Exit(1)
	}
	maxPages, err := strconv.Atoi(args[2])
	if err != nil {
		fmt.Println("MAX_PAGES must be an integer")
		os.Exit(1)
	}

	urlsFound := map[string]int{}
	numPagesCrawled := 0
	var wg sync.WaitGroup
	var mu sync.Mutex
	sem := semaphore.NewWeighted(int64(maxConcurrency))
	sem_ctx := context.TODO()
	wg.Add(1)
	go crawlPage(
		&baseURL,
		baseURL,
		urlsFound,
		&numPagesCrawled,
		maxPages,
		&mu,
		sem,
		sem_ctx,
		&wg,
	)

	wg.Wait()

	type kv struct { k string; v int }
	var kvPairs []kv
	for url, count := range urlsFound {
		kvPairs = append(kvPairs, kv { k: url, v: count })
	}
	slices.SortFunc(kvPairs, func(a, b kv) int { return b.v - a.v })

	fmt.Println()
	fmt.Printf("REPORT for %v\n", baseURL)
	fmt.Printf("crawled %v pages\n", numPagesCrawled)
	for _, entry := range kvPairs {
		fmt.Printf("\tFound %v internal links to %v\n", entry.v, entry.k)
	}
}
