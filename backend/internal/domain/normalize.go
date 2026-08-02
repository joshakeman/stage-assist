package domain

import (
	"strings"
	"unicode"
)

// Token is a single word as it appears in the source text (Surface) paired
// with a normalized form (Norm) used for comparison only.
type Token struct {
	Surface string
	Norm    string
}

// Tokenize splits text into words on whitespace and computes each word's
// normalized form. Words that normalize to nothing (pure punctuation) are
// dropped, since they carry no comparable content.
func Tokenize(text string) []Token {
	fields := strings.Fields(text)
	tokens := make([]Token, 0, len(fields))
	for _, f := range fields {
		norm := normalizeWord(f)
		if norm == "" {
			continue
		}
		tokens = append(tokens, Token{Surface: f, Norm: norm})
	}
	return tokens
}

// normalizeWord lowercases a word and strips punctuation, keeping apostrophes
// that fall inside the word (contractions) but dropping leading/trailing ones.
func normalizeWord(s string) string {
	runes := []rune(s)
	var b strings.Builder
	for i, r := range runes {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(unicode.ToLower(r))
		case r == '\'' && i > 0 && i < len(runes)-1:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// normalizeCueText reduces a full cue to a single normalized string (its
// normalized tokens, space-joined), used to decide cue-level equality so
// that whitespace/punctuation-only differences compare as identical.
func normalizeCueText(text string) string {
	tokens := Tokenize(text)
	norms := make([]string, len(tokens))
	for i, t := range tokens {
		norms[i] = t.Norm
	}
	return strings.Join(norms, " ")
}
