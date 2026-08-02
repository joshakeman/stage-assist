package domain

// DiffOp identifies how a single element relates to the transformation from
// one sequence to another.
type DiffOp string

const (
	OpEqual  DiffOp = "equal"
	OpInsert DiffOp = "insert"
	OpDelete DiffOp = "delete"
)

// opStep is one element-level operation produced by lcsDiff. AIdx is valid
// for OpEqual/OpDelete (an index into a); BIdx is valid for OpEqual/OpInsert
// (an index into b).
type opStep struct {
	Op   DiffOp
	AIdx int
	BIdx int
}

// lcsDiff computes a longest-common-subsequence diff between a and b, using
// equal to decide whether two elements match. Membership in the LCS is exact
// (by equal), with no partial or weighted matching — this is plain LCS, not
// Needleman-Wunsch: there is no substitution score and no gap penalty.
//
// It returns the sequence of per-element operations that walks a and b left
// to right, in order, using the standard dp[i][j] = LCS length of a[i:], b[j:]
// table to reconstruct the path without a separate backtracking pass.
func lcsDiff[T any](a, b []T, equal func(x, y T) bool) []opStep {
	n, m := len(a), len(b)

	dp := make([][]int, n+1)
	for i := range dp {
		dp[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			switch {
			case equal(a[i], b[j]):
				dp[i][j] = dp[i+1][j+1] + 1
			case dp[i+1][j] >= dp[i][j+1]:
				dp[i][j] = dp[i+1][j]
			default:
				dp[i][j] = dp[i][j+1]
			}
		}
	}

	steps := make([]opStep, 0, n+m)
	i, j := 0, 0
	for i < n && j < m {
		switch {
		case equal(a[i], b[j]):
			steps = append(steps, opStep{Op: OpEqual, AIdx: i, BIdx: j})
			i++
			j++
		case dp[i+1][j] >= dp[i][j+1]:
			steps = append(steps, opStep{Op: OpDelete, AIdx: i})
			i++
		default:
			steps = append(steps, opStep{Op: OpInsert, BIdx: j})
			j++
		}
	}
	for ; i < n; i++ {
		steps = append(steps, opStep{Op: OpDelete, AIdx: i})
	}
	for ; j < m; j++ {
		steps = append(steps, opStep{Op: OpInsert, BIdx: j})
	}
	return steps
}

// WordDiffSpan is a run of consecutive words sharing the same DiffOp,
// rendered as a single span of surface text.
type WordDiffSpan struct {
	Op   DiffOp
	Text string
}

// WordDiff diffs two token sequences word by word (matching on Token.Norm)
// and groups the result into surface-text spans for rendering.
func WordDiff(a, b []Token) []WordDiffSpan {
	steps := lcsDiff(a, b, func(x, y Token) bool { return x.Norm == y.Norm })

	var spans []WordDiffSpan
	for _, s := range steps {
		var text string
		if s.Op == OpInsert {
			text = b[s.BIdx].Surface
		} else {
			text = a[s.AIdx].Surface
		}
		if len(spans) > 0 && spans[len(spans)-1].Op == s.Op {
			spans[len(spans)-1].Text += " " + text
		} else {
			spans = append(spans, WordDiffSpan{Op: s.Op, Text: text})
		}
	}
	return spans
}
