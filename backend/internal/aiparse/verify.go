package aiparse

import (
	"errors"
	"strings"

	"github.com/joshakeman/stage-assist/backend/internal/domain"
	"github.com/joshakeman/stage-assist/backend/internal/pdftext"
)

// ErrNothingVerified means every element in a CandidateScript failed
// grounding -- there's nothing trustworthy to show the user at all. This is
// one of the two deliberately simple failure conditions from the ingestion
// plan (the other, a structurally-unusable response, is caught earlier by
// parseToolInput's JSON parsing). There is no aggregate rejection
// threshold beyond this -- that's a question for real evaluation data, not
// a number invented here.
var ErrNothingVerified = errors.New("aiparse: no elements could be verified against the source text")

// textEvidenceOverlapThreshold is how much of an element's Text must trace
// back to its own SourceEvidence for the pair to be trusted. This is a
// deliberately simple word-overlap check, not a similarity ranking -- see
// the package-level limitations noted in CLAUDE.md for what it doesn't
// catch (e.g. the same words reordered to change meaning).
const textEvidenceOverlapThreshold = 0.7

// Verify checks every element of script against pages and returns a copy
// with Page and Verified populated. It never drops an element -- even one
// that fails verification is returned, flagged, so the user can still
// review it -- and it returns ErrNothingVerified only when not a single
// element could be confirmed.
//
// Two independent checks decide Verified, both against text normalized for
// common PDF-extraction artifacts (see normalizeForGrounding):
//
//  1. Evidence groundedness: SourceEvidence must actually appear in the
//     extracted text of some page. This defends against a fabricated
//     evidence span.
//  2. Text-evidence consistency: Text's content words must substantially
//     overlap with SourceEvidence's. This is what closes the gap a plain
//     "does the evidence exist somewhere" check would leave open -- pairing
//     a real, trivial anchor with a long invented Text now fails here,
//     because the invented content doesn't overlap with that anchor.
func Verify(script CandidateScript, pages []pdftext.PageText) (CandidateScript, error) {
	verified := make([]CandidateElement, len(script.Elements))
	anyVerified := false

	for i, el := range script.Elements {
		page, found := findEvidencePage(el.SourceEvidence, pages)
		consistent := found && overlapRatio(contentWords(el.Text), contentWords(el.SourceEvidence)) >= textEvidenceOverlapThreshold

		el.Page = page
		el.Verified = found && consistent
		if el.Verified {
			anyVerified = true
		}
		verified[i] = el
	}

	if !anyVerified {
		return CandidateScript{}, ErrNothingVerified
	}
	return CandidateScript{Elements: verified}, nil
}

// findEvidencePage reports the first page whose normalized text contains
// normalized evidence as a substring, and that page's number. An empty
// evidence string never matches -- it isn't evidence of anything.
func findEvidencePage(evidence string, pages []pdftext.PageText) (page int, found bool) {
	needle := normalizeForGrounding(evidence)
	if needle == "" {
		return 0, false
	}
	for _, p := range pages {
		if strings.Contains(normalizeForGrounding(p.Text), needle) {
			return p.Number, true
		}
	}
	return 0, false
}

// contentWords returns s's normalized words, via domain.Tokenize -- the
// same normalization internal/domain's own comparisons already use,
// reused here rather than duplicated.
func contentWords(s string) []string {
	tokens := domain.Tokenize(s)
	words := make([]string, len(tokens))
	for i, t := range tokens {
		words[i] = t.Norm
	}
	return words
}

// overlapRatio returns the fraction of a's words that also appear in b.
// This is a bag-of-words check, not an ordering check -- two elements
// whose words are a real anagram of each other's would pass, an accepted,
// narrow limitation of a cheap heuristic (see CLAUDE.md).
func overlapRatio(a, b []string) float64 {
	if len(a) == 0 {
		return 0
	}
	set := make(map[string]bool, len(b))
	for _, w := range b {
		set[w] = true
	}
	matched := 0
	for _, w := range a {
		if set[w] {
			matched++
		}
	}
	return float64(matched) / float64(len(a))
}

// punctuationFolds maps common PDF-extraction/typographic artifacts to
// plain ASCII equivalents: smart quotes, en/em dashes, ellipses, and the
// handful of Latin ligature glyphs a script excerpt might realistically
// contain. This is a small, explicit table -- not full Unicode
// normalization -- because that's all real script PDFs need in practice.
var punctuationFolds = map[rune]string{
	'‘': "'", '’': "'", '‚': "'", '‛': "'",
	'“': `"`, '”': `"`, '„': `"`, '‟': `"`,
	'–': "-", '—': "-", '−': "-",
	'…': "...",
	'ﬀ': "ff", 'ﬁ': "fi", 'ﬂ': "fl",
	'ﬃ': "ffi", 'ﬄ': "ffl", 'ﬅ': "st", 'ﬆ': "st",
}

// normalizeForGrounding prepares text for grounding comparison: folds the
// artifacts above, rejoins a hyphen immediately followed by whitespace
// (the common mid-word line-wrap pattern, e.g. "under-\nstand"), collapses
// remaining whitespace, and lowercases. Known, accepted limitation: joining
// any hyphen-plus-whitespace this way could occasionally rejoin two
// genuinely separate hyphenated words rather than a wrapped one; this is
// judged unlikely enough in script dialogue not to special-case further.
func normalizeForGrounding(s string) string {
	var folded strings.Builder
	for _, r := range s {
		if repl, ok := punctuationFolds[r]; ok {
			folded.WriteString(repl)
		} else {
			folded.WriteRune(r)
		}
	}

	// Collapse all whitespace (including newlines) to single spaces first,
	// so "-\n", "-\t", and "-  " are all normalized to the same "- " shape
	// before the single hyphen-rejoining rule below.
	collapsed := strings.Join(strings.Fields(folded.String()), " ")
	dehyphenated := strings.ReplaceAll(collapsed, "- ", "")

	return strings.ToLower(dehyphenated)
}
