package index

import (
	"testing"
)

// TestTokenizer tests the tokenizer functionality
func TestTokenizer(t *testing.T) {
	// Create tokenizer
	tokenizer := NewUnicodeTokenizer()

	// Test cases
	tests := []struct {
		name        string
		text        string
		wantTokens  []string
		description string
	}{
		{
			name:        "English text tokenization",
			text:        "Hello world, this is a test.",
			wantTokens:  []string{"hello", "world", "this", "test"},
			description: "Test basic English text tokenization",
		},
		{
			name:        "Mixed text tokenization",
			text:        "Hello 世界, this is a test.",
			wantTokens:  []string{"hello", "世界", "this", "test"},
			description: "Test mixed text tokenization",
		},
		{
			name:        "Number handling",
			text:        "Test123 numbers456",
			wantTokens:  []string{"test", "123", "numbers", "456"},
			description: "Test number handling",
		},
		{
			name:        "Stop word handling",
			text:        "the cat is on the mat",
			wantTokens:  []string{"cat", "mat"},
			description: "Test stop word handling",
		},
		{
			name:        "Empty text handling",
			text:        "",
			wantTokens:  []string{},
			description: "Test empty text handling",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens, err := tokenizer.Tokenize(tt.text)
			if err != nil {
				t.Errorf("Tokenize() error = %v", err)
				return
			}

			// Extract token text
			gotTokens := make([]string, len(tokens))
			for i, token := range tokens {
				gotTokens[i] = token.Text
			}

			// Check result count
			if len(gotTokens) != len(tt.wantTokens) {
				t.Errorf("Tokenize() got %d tokens, want %d tokens", len(gotTokens), len(tt.wantTokens))
				return
			}

			// Check result content
			for i := range gotTokens {
				if gotTokens[i] != tt.wantTokens[i] {
					t.Errorf("Tokenize() gotTokens[%d] = %s, want %s", i, gotTokens[i], tt.wantTokens[i])
				}
			}
		})
	}
}

// TestTokenizerOptions tests tokenizer options
func TestTokenizerOptions(t *testing.T) {
	tests := []struct {
		name        string
		options     []TokenizerOption
		text        string
		wantTokens  []string
		description string
	}{
		{
			name: "Case sensitivity",
			options: []TokenizerOption{
				WithCaseSensitive(true),
			},
			text:        "Hello HELLO hello",
			wantTokens:  []string{"Hello", "HELLO", "hello"},
			description: "Test case sensitivity option",
		},
		{
			name: "Number preservation",
			options: []TokenizerOption{
				WithPreserveNumbers(true),
			},
			text:        "Test123 numbers456",
			wantTokens:  []string{"test", "123", "numbers", "456"},
			description: "Test number preservation option",
		},
		{
			name: "Punctuation preservation",
			options: []TokenizerOption{
				WithPreservePunctuation(true),
			},
			text:        "Hello, world!",
			wantTokens:  []string{"Hello", ",", "world", "!"},
			description: "Test punctuation preservation option",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokenizer := NewUnicodeTokenizer(tt.options...)
			tokens, err := tokenizer.Tokenize(tt.text)
			if err != nil {
				t.Errorf("Tokenize() error = %v", err)
				return
			}

			// Extract token text
			gotTokens := make([]string, len(tokens))
			for i, token := range tokens {
				gotTokens[i] = token.Text
			}

			// Check result count
			if len(gotTokens) != len(tt.wantTokens) {
				t.Errorf("Tokenize() got %d tokens, want %d tokens", len(gotTokens), len(tt.wantTokens))
				return
			}

			// Check result content
			for i := range gotTokens {
				if gotTokens[i] != tt.wantTokens[i] {
					t.Errorf("Tokenize() gotTokens[%d] = %s, want %s", i, gotTokens[i], tt.wantTokens[i])
				}
			}
		})
	}
}

// TestAddStopWords tests adding stop words
func TestAddStopWords(t *testing.T) {
	tokenizer := NewUnicodeTokenizer()

	// Add custom stop words
	customStopWords := []string{"custom", "stop", "word"}
	tokenizer.AddStopWords(customStopWords)

	// Test text
	text := "this is a custom stop word test"
	tokens, err := tokenizer.Tokenize(text)
	if err != nil {
		t.Errorf("Tokenize() error = %v", err)
		return
	}

	// Extract token text
	gotTokens := make([]string, len(tokens))
	for i, token := range tokens {
		gotTokens[i] = token.Text
	}

	// Check that stop words are not in results
	for _, token := range gotTokens {
		for _, stopWord := range customStopWords {
			if token == stopWord {
				t.Errorf("Tokenize() result contains stop word: %s", stopWord)
			}
		}
	}
}
