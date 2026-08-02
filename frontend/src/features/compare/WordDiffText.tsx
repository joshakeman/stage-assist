import type { WordDiffSpan } from "../../api/types";

interface WordDiffTextProps {
  spans: WordDiffSpan[];
  side: "script" | "spoken";
}

// Renders one side of a word-level diff: the script side strikes deleted
// words, the spoken side highlights inserted words, and words that belong
// only to the other side are omitted from this side's rendering.
export function WordDiffText({ spans, side }: WordDiffTextProps) {
  const omitOp = side === "script" ? "insert" : "delete";

  return (
    <>
      {spans
        .filter((span) => span.op !== omitOp)
        .map((span, i) => (
          <span
            key={i}
            className={span.op === "equal" ? undefined : `word-diff-${span.op}`}
          >
            {span.text}{" "}
          </span>
        ))}
    </>
  );
}
