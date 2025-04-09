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
