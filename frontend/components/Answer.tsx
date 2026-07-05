// Answer shows the model's direct answer to the query, with its source file.
export function Answer({ answer, source }: { answer: string; source?: string }) {
  return (
    <div className="answer">
      <p className="answer-text">{answer}</p>
      {source && <p className="answer-source">from {source}</p>}
    </div>
  );
}
