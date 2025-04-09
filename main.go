package main

import "fmt"

func main() {
	testHTML := `
<html>
	<body>
		<a href="/path/one">
			<span>Boot.dev</span>
		</a>
		<a href="https://other.com/path/one">
			<span>Boot.dev</span>
		</a>
		<div>
			<a href="/hello">hello</a>
			<a href="/world">world</a>
		</div>
	</body>
</html>
	`

	result, _ := getURLsFromHTML(testHTML, "test.com")
	fmt.Printf("%v\n", result)
}
