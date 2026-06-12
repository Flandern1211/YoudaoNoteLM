import { useState, useEffect } from 'react';
import { Loader2, Check, AlertCircle, ArrowLeft } from 'lucide-react';
import { cn } from '../../utils/cn';
import * as youdaoApi from '../../api/youdao';

interface YoudaoImportPanelProps {
  onImport: (fileIds: string[], fileNames: Record<string, string>) => Promise<void>;
  onBack: () => void;
}

export default function YoudaoImportPanel({ onImport, onBack }: YoudaoImportPanelProps) {
  const [notebooks, setNotebooks] = useState<youdaoApi.YoudaoNotebook[]>([]);
  const [loading, setLoading] = useState(true);
  const [importing, setImporting] = useState(false);
  const [selectedNotes, setSelectedNotes] = useState<string[]>([]);
  const [error, setError] = useState('');

  useEffect(() => {
    loadNotebooks();
  }, []);

  const loadNotebooks = async () => {
    try {
      setLoading(true);
      const res = await youdaoApi.listNotebooks();
      if (res.code === 0) {
        setNotebooks(res.data);
      } else {
        setError(res.message || '加载有道云笔记失败');
      }
    } catch (err) {
      setError('加载有道云笔记失败');
    } finally {
      setLoading(false);
    }
  };

  const handleToggleNote = (noteId: string) => {
    setSelectedNotes((prev) =>
      prev.includes(noteId) ? prev.filter((id) => id !== noteId) : [...prev, noteId]
    );
  };

  const handleImport = async () => {
    if (selectedNotes.length === 0) return;

    try {
      setImporting(true);
      setError('');

      const fileNames: Record<string, string> = {};
      selectedNotes.forEach((id) => {
        for (const nb of notebooks) {
          const note = nb.notes.find((n) => n.note_id === id);
          if (note) {
            fileNames[id] = note.title;
            break;
          }
        }
      });

      await onImport(selectedNotes, fileNames);
    } catch (err) {
      setError('导入失败');
    } finally {
      setImporting(false);
    }
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center p-8">
        <Loader2 size={24} className="animate-spin text-accent" />
        <span className="ml-2 text-sm text-text-muted">加载有道云笔记...</span>
      </div>
    );
  }

  return (
    <div className="flex flex-col h-full">
      <div className="flex items-center gap-2 px-4 py-3 border-b border-border">
        <button onClick={onBack} className="p-1 hover:bg-bg-hover rounded cursor-pointer">
          <ArrowLeft size={16} className="text-text-secondary" />
        </button>
        <h3 className="text-sm font-medium text-text-primary">导入有道云笔记</h3>
      </div>

      {error && (
        <div className="mx-4 mt-3 p-2 bg-error/10 text-error text-xs rounded flex items-center gap-1.5">
          <AlertCircle size={12} />
          {error}
        </div>
      )}

      <div className="flex-1 overflow-y-auto p-4 space-y-4">
        {notebooks.map((nb) => (
          <div key={nb.notebook_id} className="space-y-2">
            <h4 className="text-xs font-medium text-text-secondary">{nb.name}</h4>
            <div className="space-y-1">
              {nb.notes.map((note) => (
                <label
                  key={note.note_id}
                  className={cn(
                    'flex items-center gap-2 p-2 rounded-lg cursor-pointer transition-colors',
                    selectedNotes.includes(note.note_id) ? 'bg-accent/10' : 'hover:bg-bg-hover'
                  )}
                >
                  <input
                    type="checkbox"
                    checked={selectedNotes.includes(note.note_id)}
                    onChange={() => handleToggleNote(note.note_id)}
                    className="sr-only"
                  />
                  <div
                    className={cn(
                      'w-4 h-4 rounded border flex items-center justify-center',
                      selectedNotes.includes(note.note_id)
                        ? 'bg-accent border-accent'
                        : 'border-border-light'
                    )}
                  >
                    {selectedNotes.includes(note.note_id) && <Check size={10} className="text-white" />}
                  </div>
                  <span className="text-xs text-text-primary truncate">{note.title}</span>
                </label>
              ))}
            </div>
          </div>
        ))}
      </div>

      <div className="p-4 border-t border-border">
        <button
          onClick={handleImport}
          disabled={selectedNotes.length === 0 || importing}
          className={cn(
            'w-full py-2 rounded-lg text-sm font-medium transition-colors cursor-pointer',
            selectedNotes.length > 0 && !importing
              ? 'bg-accent text-white hover:bg-accent-light'
              : 'bg-bg-hover text-text-muted cursor-not-allowed'
          )}
        >
          {importing ? (
            <span className="flex items-center justify-center gap-2">
              <Loader2 size={14} className="animate-spin" />
              导入中...
            </span>
          ) : (
            `导入选中的 ${selectedNotes.length} 篇笔记`
          )}
        </button>
      </div>
    </div>
  );
}
