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
)

func normalizeURL(url string) (string, error) {
	parsed, err := neturl.Parse(url)
	if err != nil { return "", err }
	return strings.TrimRight(parsed.Host + parsed.Path, "/"), nil
}

func getURLsFromHTML(htmlBody, rawBaseURL string) ([]string, error) {
	var result []string

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
					if attr.Val[0] == '/' {
						if rawBaseURL[len(rawBaseURL) - 1] == '/' {
							result = append(result, rawBaseURL + attr.Val[1:])
						} else {
							result = append(result, rawBaseURL + attr.Val)
						}
					} else {
						result = append(result, attr.Val)
					}
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
	resp, err := http.Get(rawURL)
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
	pagesVisited map[string]int,
	pagesCrawled *int,
	MAX_PAGES int,
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

	fmt.Printf("%v - acquiring mutex\n", urlToCrawl)
	mu.Lock()
	fmt.Printf("%v - acquired mutex\n", urlToCrawl)
	if _, exists := pagesVisited[normalizedUrl]; exists {
		pagesVisited[normalizedUrl] += 1
		mu.Unlock()
		return // no need to fetch and crawl again
	} else {
		pagesVisited[normalizedUrl] = 1
	}
	*pagesCrawled += 1
	if *pagesCrawled >= MAX_PAGES {
		mu.Unlock()
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
			go crawlPage(base, discoveredUrl, pagesVisited, pagesCrawled, MAX_PAGES, mu, sem, sem_ctx, wg)
		} else {
			fmt.Printf("\t%v: %v - %v DISCARD\n", i, urlToCrawl, discoveredUrl)
		}
	}

	fmt.Printf("%v - EXIT\n", urlToCrawl)
}

func main() {
	args := os.Args[1:]
	if len(args) != 2 {
		fmt.Println("usage: ./crawler URL MAX_CONCURRENCY")
		os.Exit(1)
	}

	baseURL := args[0]
	maxConcurrency, err := strconv.Atoi(args[1])
	if err != nil {
		fmt.Println("MAX_CONCURRENCY must be an integer")
		os.Exit(1)
	}

	MAX_PAGES := 20
	pagesVisited := map[string]int{}
	pagesCrawled := 0

	var wg sync.WaitGroup
	var mu sync.Mutex
	sem := semaphore.NewWeighted(int64(maxConcurrency))
	sem_ctx := context.TODO()
	wg.Add(1)
	go crawlPage(
		&baseURL,
		baseURL,
		pagesVisited,
		&pagesCrawled,
		MAX_PAGES,
		&mu,
		sem,
		sem_ctx,
		&wg,
	)

	wg.Wait()

	fmt.Printf("found %v urls\n", pagesCrawled)
	for k, v := range pagesVisited {
		fmt.Printf("\t%v - %v\n", k, v)
	}
}
