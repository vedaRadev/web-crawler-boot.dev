package main

import "testing"

func TestNormalizeURL(t *testing.T) {
	tests := []struct {
		name          string
		inputURL      string
		expected      string
	}{
		{
			name:     "remove scheme",
			inputURL: "https://blog.boot.dev/path",
			expected: "blog.boot.dev/path",
		},
		{
			name: "multi path",
			inputURL: "http://myurl.com/multi/path/hello",
			expected: "myurl.com/multi/path/hello",
		},
		{
			name: "extraneous slashes removed",
			inputURL: "http://hello.com/test/////",
			expected: "hello.com/test",
		},
		{
			name: "no path",
			inputURL: "http://hello.com",
			expected: "hello.com",
		},
		{
			name: "no scheme",
			inputURL: "hello.com/path",
			expected: "hello.com/path",
		},
		{
			name: "with port",
			inputURL: "http://localhost:8080/path",
			expected: "localhost:8080/path",
		},
	}

	for i, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actual, err := normalizeURL(tc.inputURL)
			if err != nil {
				t.Errorf("Test %v - '%s' FAIL: unexpected error: %v", i, tc.name, err)
				return
			}
			if actual != tc.expected {
				t.Errorf("Test %v - %s FAIL: expected URL: %v, actual: %v", i, tc.name, tc.expected, actual)
			}
		})
	}
}

func TestGetURLsFromHTML(t *testing.T) {
	tests := []struct {
		name string
		inputURL string
		inputHTML string
		expected []string
	}{
		{
			name: "basic",
			inputURL: "https://blog.boot.dev",
			inputHTML: `
<html>
	<body>
		<a href="/path/one">
			<span>Boot.dev</span>
		</a>
		<a href="https://other.com/path/one">
			<span>Boot.dev</span>
		</a>
	</body>
</html>
			`,
			expected: []string{"https://blog.boot.dev/path/one", "https://other.com/path/one"},
		},
		{
			name: "empty href",
			inputURL: "https://test.com",
			inputHTML: `
<html>
	<body>
		<a href="">Test1</a>
		<a href="">Test2</a>
		<a href="">Test3</a>
	</body>
</html>
			`,
			expected: []string{},
		},
		{
			name: "many anchors",
			inputURL: "https://test.com",
			inputHTML: `
<html>
	<body>
		<a href="/hello">hello</a>
		<div>
			<a href="/world">world</a>
			<a href="http://something.com/test">something</a>
			<div>
				<div>
					<span>test</span>
					<div>
						<a href="/deepnest">deepnest</a>
						<a href="https://other.com">other</a>
					</div>
				</div>
			</div>
		</div>
		<div>
			<a href="/another">another</a>
		</div>
	</body>
</html>
			`,
			expected: []string{
				"https://test.com/hello",
				"https://test.com/world",
				"http://something.com/test",
				"https://test.com/deepnest",
				"https://other.com",
				"https://test.com/another",
			},
		},
	}

	for i, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actual, err := getURLsFromHTML(tc.inputHTML, tc.inputURL)
			if err != nil {
				t.Errorf("Test %v - '%s' FAIL: unexpected error: %v", i, tc.name, err)
				return
			}
			passes := len(actual) == len(tc.expected)
			if passes {
				for i := range len(actual) {
					if actual[i] != tc.expected[i] {
						passes = false
						break
					}
				}
			}
			if !passes {
				t.Errorf("Test %v - %s FAIL: expected %v but got %v", i, tc.name, tc.expected, actual)
			}
		})
	}
}
