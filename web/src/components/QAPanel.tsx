import { useState, useRef, useEffect } from 'preact/hooks';
import { renderMarkdown } from '../lib/markdown';
import { streamQuery } from '../lib/sse';

interface Props {
  onNavigate: (path: string) => void;
}

export function QAPanel({ onNavigate }: Props) {
  const [question, setQuestion] = useState('');
  const [answer, setAnswer] = useState('');
  const [sources, setSources] = useState<string[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const controllerRef = useRef<AbortController | null>(null);

  // Abort streaming on unmount
  useEffect(() => {
    return () => { controllerRef.current?.abort(); };
  }, []);

  const handleSubmit = (e: Event) => {
    e.preventDefault();
    if (!question.trim() || loading) return;

    // Cancel previous request
    controllerRef.current?.abort();

    setAnswer('');
    setSources([]);
    setError(null);
    setLoading(true);

    controllerRef.current = streamQuery(question, 5, {
      onToken: (text) => {
        setAnswer(prev => prev + text);
      },
      onSources: (paths) => {
        setSources(paths);
      },
      onDone: () => {
        setLoading(false);
      },
      onError: (err) => {
        setError(err);
        setLoading(false);
      },
    });
  };

  const handleSourceClick = (path: string) => {
    // Convert wiki/concepts/foo.md → concepts/foo.md
    const articlePath = path.replace(/^wiki\//, '');
    onNavigate(articlePath);
  };

  const handleLinkClick = (e: MouseEvent) => {
    const target = e.target as HTMLElement;
    const link = target.closest('a');
    if (link?.getAttribute('href')?.startsWith('/wiki/')) {
      e.preventDefault();
      const path = link.getAttribute('href')!.replace('/wiki/', '') + '.md';
      onNavigate(path);
    }
  };

  return (
    <div class="border-t border-gray-200 dark:border-gray-700 flex flex-col max-h-[40vh]">
      {/* Answer area */}
      {(answer || loading || error) && (
        <div class="flex-1 overflow-y-auto px-6 py-4 min-h-0">
          {error && (
            <div class="text-red-500 text-sm mb-2">Error: {error}</div>
          )}
          {answer && (
            <div
              class="prose dark:prose-invert prose-sm max-w-none"
              onClick={handleLinkClick}
              dangerouslySetInnerHTML={{ __html: renderMarkdown(answer) }}
            />
          )}
          {loading && !answer && (
            <div class="flex items-center gap-2 text-gray-400 text-sm">
              <span class="animate-pulse">Thinking...</span>
            </div>
          )}
          {loading && answer && (
            <span class="inline-block w-2 h-4 bg-blue-500 animate-pulse ml-0.5" />
          )}

          {/* Sources */}
          {sources.length > 0 && (
            <div class="mt-3 pt-3 border-t border-gray-200 dark:border-gray-700">
              <span class="text-xs text-gray-500 font-medium">Sources: </span>
              {sources.map((s, i) => (
                <button
                  key={s}
                  onClick={() => handleSourceClick(s)}
                  class="text-xs text-blue-600 dark:text-blue-400 hover:underline"
                >
                  {s.split('/').pop()?.replace('.md', '')}
                  {i < sources.length - 1 && ', '}
                </button>
              ))}
            </div>
          )}
        </div>
      )}

      {/* Input */}
      <form onSubmit={handleSubmit} class="flex gap-2 px-4 py-3 border-t border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-800">
        <input
          type="text"
          value={question}
          onInput={(e) => setQuestion((e.target as HTMLInputElement).value)}
          placeholder="Ask a question about the wiki..."
          disabled={loading}
          class="flex-1 px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-900 text-gray-900 dark:text-white focus:ring-2 focus:ring-blue-500 focus:border-transparent disabled:opacity-50"
        />
        <button
          type="submit"
          disabled={loading || !question.trim()}
          class="px-4 py-2 text-sm bg-blue-600 text-white rounded-md hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed"
        >
          {loading ? '...' : 'Ask'}
        </button>
      </form>
    </div>
  );
}
