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

func crawlPage(base, urlToCrawl string, discoveredUrls chan <-string, sem *semaphore.Weighted) {
	defer sem.Release(1)
	defer fmt.Printf("%v - releasing semaphore\n", urlToCrawl)

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
		if strings.Contains(discoveredUrl, base) {
			fmt.Printf("\t%v: %v - SEND %v\n", i, urlToCrawl, discoveredUrl)
			discoveredUrls <- discoveredUrl
		} else {
			fmt.Printf("\t%v: %v - DISCARD %v\n", i, urlToCrawl, discoveredUrl)
		}
	}
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

	normalizedBase, err := normalizeURL(baseURL)
	if err != nil {
		fmt.Printf("failed to normalize %v: %v\n", baseURL, err)
		os.Exit(1)
	}

	// FIXME this could potentially fill up and our crawlers won't be able to send any more
	// messages, meaning either data will be dropped or the crawler will block and won't
	// release its semaphore, ensuring program deadlock. Can mitigate this by switching to a
	// recursive crawl model.
	discoveredUrls := make(chan string, 8192)
	var wg sync.WaitGroup
	wg.Add(1)
	availableWorkers := semaphore.NewWeighted(int64(maxConcurrency))
	ctx := context.TODO()
	go (func() {
		pagesVisited := map[string]int{}
		const MAX_PAGES int = 20
		pagesCrawled := 0
		for url := range discoveredUrls {
			fmt.Println()
			fmt.Printf("visiting: %v\n", url)
			normalizedUrl, err := normalizeURL(url)
			if err != nil {
				fmt.Printf("failed to normalize %v: %v\n", url, err)
				continue
			}
			fmt.Printf("normalized: %v\n", normalizedUrl)
			if _, exists := pagesVisited[normalizedUrl]; exists {
				pagesVisited[normalizedUrl] += 1
				continue // no need to fetch and crawl again
			} else {
				pagesVisited[normalizedUrl] = 1
			}

			pagesCrawled += 1
			if pagesCrawled >= MAX_PAGES { break }

			fmt.Printf("acquiring goroutine for crawl of %v\n", url)
			err = availableWorkers.Acquire(ctx, 1)
			if err != nil {
				fmt.Printf("failed to acquire goroutine for %v: %v\n", url, err)
				os.Exit(1)
			}

			fmt.Printf("ACQUIRED for %v\n", url)
			go crawlPage(normalizedBase, url, discoveredUrls, availableWorkers)
		}

		fmt.Println("closing channel")
		close(discoveredUrls)
		fmt.Printf("visited %v urls\n", pagesCrawled)
		for k, v := range pagesVisited {
			fmt.Printf("\t%v - %v\n", k, v)
		}
		wg.Done()
	})()

	discoveredUrls <- baseURL
	wg.Wait()
	fmt.Println("end")
}
