package main

import (
	"os"
	"fmt"
	"io"
	"net/http"
	neturl "net/url"
	nethtml "golang.org/x/net/html"
	"strings"
	"time"
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

type SimpleQueueNode[T any] struct {
	val T
	next *SimpleQueueNode[T]
}

type SimpleQueue[T any] struct {
	front *SimpleQueueNode[T]
	back *SimpleQueueNode[T]
}

// Add to back
func (q *SimpleQueue[T]) Add(val T) {
	node := SimpleQueueNode[T] { val: val }
	if q.front == nil {
		q.front = &node
		q.back = &node
	} else {
		q.back.next = &node
		q.back = &node
	}
}

// Pop from front
func (q *SimpleQueue[T]) Pop() (T, bool) {
	var result T
	if q.front == nil { return result, false }
	node := q.front
	q.front = q.front.next
	if q.front == nil { q.back = nil }
	return node.val, true
}

func (q *SimpleQueue[T]) IsEmpty() bool {
	return q.front == nil
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

	pagesVisited := map[string]int{}
	visitQueue := SimpleQueue[string]{}
	visitQueue.Add(baseURL)
	for !visitQueue.IsEmpty() {
		urlToCrawl, _ := visitQueue.Pop()
		if !strings.Contains(urlToCrawl, normalizedBase) {
			continue
		}
		fmt.Println()
		fmt.Printf("visiting: %v\n", urlToCrawl)
		normalizedUrl, err := normalizeURL(urlToCrawl)
		if err != nil {
			fmt.Printf("failed to normalize %v: %v\n", urlToCrawl, err)
			continue
		}
		fmt.Printf("normalized: %v\n", normalizedUrl)
		if _, exists := pagesVisited[normalizedUrl]; exists {
			pagesVisited[normalizedUrl] += 1
			continue // no need to fetch and crawl again
		} else {
			pagesVisited[normalizedUrl] = 1
		}

		fmt.Printf("fetching HTML for: %v\n", urlToCrawl)
		time.Sleep(100 * time.Millisecond)
		html, err := getHTML(urlToCrawl)
		if err != nil {
			fmt.Printf("failed to fetch HTML for %v: %v\n", urlToCrawl, err)
			continue
		}

		fmt.Printf("starting crawl of: %v\n", urlToCrawl)
		discovered, err := getURLsFromHTML(html, urlToCrawl)
		if err != nil {
			fmt.Printf("failed to crawl %v: %v\n", urlToCrawl, err)
			continue
		}

		fmt.Println("DISCOVERED:")
		for _, discoveredUrl := range discovered {
			fmt.Printf("\t%v\n", discoveredUrl)
			visitQueue.Add(discoveredUrl)
		}
	}

	fmt.Println(pagesVisited)
}
