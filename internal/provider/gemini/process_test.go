package gemini

import "testing"

func TestIsGeminiProcess(t *testing.T) {
	testCaseList := []struct {
		name    string
		argLine string
		want    bool
	}{
		{"exact binary", "gemini --resume", true},
		{"full path", "/usr/local/bin/gemini chat", true},
		{"npx path", "/home/user/.npm/bin/gemini", true},
		{"not gemini - grep", "/usr/bin/grep gemini", false},
		{"not gemini - ps", "/bin/ps -eo pid", false},
		{"not gemini - gemini-pro", "gemini-pro serve", false},
		{"not gemini - node", "node gemini-server.js", false},
		{"not gemini - empty", "", false},
		{"bare gemini", "gemini", true},
	}

	for _, tc := range testCaseList {
		t.Run(tc.name, func(t *testing.T) {
			got := isGeminiProcess(tc.argLine)
			if got != tc.want {
				t.Errorf("isGeminiProcess(%q) = %v, want %v", tc.argLine, got, tc.want)
			}
		})
	}
}
