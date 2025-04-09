package main

import (
	neturl "net/url"
	nethtml "golang.org/x/net/html"
	"strings"
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
