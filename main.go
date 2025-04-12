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
						result = append(result, rawBaseURL + attr.Val)
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

func crawlPage(urlToCrawl string, discoveredUrls chan <-string) {
	html, err := getHTML(urlToCrawl)
	if err != nil {
		fmt.Printf("failed to fetch HTML for %v: %v\n", urlToCrawl, err)
		return
	}

	fmt.Printf("starting crawl of: %v\n", urlToCrawl)
	discovered, err := getURLsFromHTML(html, urlToCrawl)
	if err != nil {
		fmt.Printf("failed to crawl %v: %v\n", urlToCrawl, err)
		return
	}

	for _, discoveredUrl := range discovered {
		discoveredUrls <- discoveredUrl
	}
}

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		fmt.Println("no website provided")
		os.Exit(1)
	}
	if len(args) > 1 {
		fmt.Println("too many arguments provided")
		os.Exit(1)
	}

	baseURL := args[0]
	normalizedBase, err := normalizeURL(baseURL)
	if err != nil {
		fmt.Printf("failed to normalize %v: %v\n", baseURL, err)
		os.Exit(1)
	}

	discoveredUrls := make(chan string)
	var wg sync.WaitGroup
	go (func() {
		wg.Add(1)
		pagesVisited := map[string]int{}
		const MAX_PAGES int = 20
		pagesCrawled := 0
		for url := range discoveredUrls {
			// Limit our crawl to a single domain
			if !strings.Contains(url, normalizedBase) { continue; }
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

			// crawl the page
			// TODO limit the number of goroutines we can spawn here.
			// Maybe have a goroutine pool or something.
			go crawlPage(url, discoveredUrls)
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
}
