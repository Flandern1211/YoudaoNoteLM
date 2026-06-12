import { create } from 'zustand';
import type { Notebook, Source, Conversation, Note } from '../types';
import * as notebookApi from '../api/notebook';
import * as sourceApi from '../api/source';
import * as importApi from '../api/import';
import * as searchApi from '../api/search';
import * as chatApi from '../api/chat';
import { getErrorMessage } from '../utils/error';

interface NotebookState {
  notebooks: Notebook[];
  currentNotebookId: string | null;
  currentConversationId: string | null;
  loading: boolean;
  // sourceID → taskID 映射，用于取消正在运行的导入任务
  taskIdBySourceId: Record<string, string>;

  // Init - fetch from API
  fetchNotebooks: () => Promise<void>;

  // Getters
  getCurrentNotebook: () => Notebook | undefined;
  getCurrentConversation: () => Conversation | undefined;
  getSelectedSources: () => Source[];

  // Notebook actions (API)
  setCurrentNotebook: (id: string) => Promise<void>;
  createNotebook: (name: string) => Promise<void>;
  deleteNotebook: (id: string) => Promise<void>;
  renameNotebook: (id: string, name: string) => Promise<void>;

  // Source actions (API-backed)
  fetchSources: (notebookId: string, page?: number, size?: number, keyword?: string) => Promise<void>;
  addSource: (notebookId: string, source: Source) => void;
  updateSource: (notebookId: string, sourceId: string, updates: Partial<Source>) => void;
  removeSource: (notebookId: string, sourceId: string) => Promise<void>;
  batchRemoveSources: (notebookId: string, sourceIds: string[]) => Promise<void>;
  toggleSourceSelection: (notebookId: string, sourceId: string) => void;
  renameSource: (notebookId: string, sourceId: string, name: string) => Promise<void>;
  selectAllSources: (notebookId: string) => void;
  deselectAllSources: (notebookId: string) => void;
  fetchSourceContent: (notebookId: string, sourceId: string) => Promise<string>;
  fetchSourceOriginal: (notebookId: string, sourceId: string) => Promise<{ content: string; type: string }>;
  getSourceDownloadURL: (notebookId: string, sourceId: string) => Promise<string>;

  // Import actions (API)
  importFile: (notebookId: string, file: File) => Promise<Source>;
  previewAudio: (notebookId: string, file: File) => Promise<importApi.AudioPreviewData>;
  pollAudioPreview: (previewId: string) => Promise<importApi.AudioPreviewStatusData>;
  confirmAudio: (previewId: string, notebookId: string, content?: string) => Promise<Source>;
  getImportTask: (taskId: string) => Promise<importApi.ImportTaskData>;
  deleteImportTask: (taskId: string) => Promise<void>;

  // Search actions (API)
  searchSources: (notebookId: string, query: string) => Promise<searchApi.SearchResponseData>;
  searchSourcesStream: (
    notebookId: string,
    query: string,
    onEvent: (event: searchApi.SearchStreamEvent) => void,
    signal?: AbortSignal,
  ) => Promise<void>;
  importFromURL: (notebookId: string, url: string) => Promise<{ taskId: string; sourceId: number }>;
  importSearchResults: (notebookId: string, items: searchApi.SearchImportItem[]) => Promise<{ taskId: string; sourceIds: number[] }>;

  // Conversation actions (API-backed)
  fetchConversations: (notebookId: string) => Promise<void>;
  createConversation: (notebookId: string) => Promise<string | null>;
  setCurrentConversation: (id: string) => void;
  deleteConversation: (notebookId: string, conversationId: string) => Promise<void>;
  addMessage: (notebookId: string, conversationId: string, message: any) => void;

  // Note actions (local)
  addNote: (notebookId: string, note: Note) => void;
  deleteNote: (notebookId: string, noteId: string) => void;
  renameNote: (notebookId: string, noteId: string, title: string) => void;
  updateNoteContent: (notebookId: string, noteId: string, content: string) => void;
  toggleNoteSource: (notebookId: string, noteId: string) => void;
}

// Helper: convert backend SourceData to frontend Source
function toSource(s: sourceApi.SourceData): Source {
  // 后端状态映射：pending/processing → loading, ready → ready, failed → error
  let status: 'loading' | 'ready' | 'error' | undefined;
  if (s.status === 'pending' || s.status === 'processing') {
    status = 'loading';
  } else if (s.status === 'ready') {
    status = 'ready';
  } else if (s.status === 'failed') {
    status = 'error';
  }

  return {
    id: String(s.id),
    name: s.name,
    type: s.type === 'note' ? 'youdao' : s.type,
    size: s.file_size || undefined,
    url: s.original_url || undefined,
    selected: true, // default selected
    status,
    errorMessage: s.error_message || undefined,
    createdAt: s.created_at,
    updatedAt: s.updated_at,
  };
}

export const useNotebookStore = create<NotebookState>((set, get) => ({
  notebooks: [],
  currentNotebookId: null,
  currentConversationId: null,
  taskIdBySourceId: {},
  loading: false,

  fetchNotebooks: async () => {
    set({ loading: true });
    try {
      const res = await notebookApi.listNotebooks();
      if (res.code === 0) {
        const notebooks: Notebook[] = res.data.map((nb) => ({
          id: String(nb.id),
          name: nb.name,
          sources: [],
          conversations: [],
          notes: [],
          createdAt: nb.created_at,
          updatedAt: nb.updated_at,
        }));
        set({ notebooks });
      }
    } catch (err) {
      console.error('Failed to fetch notebooks:', err);
    } finally {
      set({ loading: false });
    }
  },

  getCurrentNotebook: () => {
    const { notebooks, currentNotebookId } = get();
    return notebooks.find((n) => n.id === currentNotebookId);
  },

  getCurrentConversation: () => {
    const notebook = get().getCurrentNotebook();
    if (!notebook) return undefined;
    const { currentConversationId } = get();
    return notebook.conversations.find((c) => c.id === currentConversationId);
  },

  getSelectedSources: () => {
    const notebook = get().getCurrentNotebook();
    if (!notebook) return [];
    return notebook.sources.filter((s) => s.selected);
  },

  setCurrentNotebook: async (id) => {
    set({
      currentNotebookId: id,
      currentConversationId: null,
    });
    // 并行加载 sources 和 conversations
    await Promise.all([
      get().fetchSources(id),
      get().fetchConversations(id),
    ]);
  },

  createNotebook: async (name) => {
    try {
      const res = await notebookApi.createNotebook(name);
      if (res.code === 0) {
        const newNotebook: Notebook = {
          id: String(res.data.id),
          name: res.data.name,
          sources: [],
          conversations: [],
          notes: [],
          createdAt: res.data.created_at,
          updatedAt: res.data.updated_at,
        };
        set((state) => ({
          notebooks: [newNotebook, ...state.notebooks],
          currentNotebookId: newNotebook.id,
        }));
      } else {
        throw new Error(res.message);
      }
    } catch (err: any) {
      if (err?.response?.status === 409) {
        throw new Error(err.response.data?.message || '已存在同名笔记本');
      }
      throw err;
    }
  },

  deleteNotebook: async (id) => {
    try {
      const res = await notebookApi.deleteNotebook(Number(id));
      if (res.code === 0) {
        set((state) => {
          const filtered = state.notebooks.filter((n) => n.id !== id);
          return {
            notebooks: filtered,
            currentNotebookId:
              state.currentNotebookId === id ? (filtered[0]?.id ?? null) : state.currentNotebookId,
          };
        });
      }
    } catch (err) {
      console.error('Failed to delete notebook:', err);
      throw err;
    }
  },

  renameNotebook: async (id, name) => {
    try {
      const res = await notebookApi.renameNotebook(Number(id), name);
      if (res.code === 0) {
        set((state) => ({
          notebooks: state.notebooks.map((n) =>
            n.id === id ? { ...n, name, updatedAt: new Date().toISOString() } : n
          ),
        }));
      } else {
        throw new Error(res.message);
      }
    } catch (err: any) {
      if (err?.response?.status === 409) {
        throw new Error(err.response.data?.message || '已存在同名笔记本');
      }
      throw err;
    }
  },

  // ---- Source actions (API-backed) ----

  fetchSources: async (notebookId, page = 1, size = 50, keyword) => {
    try {
      const res = await sourceApi.listSources(Number(notebookId), { page, size, keyword });
      if (res.code === 0) {
        const serverSources = res.data.list.map(toSource);
        set((state) => {
          const notebook = state.notebooks.find(n => n.id === notebookId);
          if (!notebook) return state;

          // 保留本地的 loading/error 状态的 placeholder（ID 以 loading- 开头）
          const localPlaceholders = notebook.sources.filter(s =>
            s.id.startsWith('loading-')
          );

          // 合并：服务器数据 + 本地 placeholder
          const mergedSources = [...serverSources, ...localPlaceholders];

          return {
            notebooks: state.notebooks.map((n) =>
              n.id === notebookId ? { ...n, sources: mergedSources } : n
            ),
          };
        });
      }
    } catch (err) {
      console.error('Failed to fetch sources:', err);
    }
  },

  addSource: (notebookId, source) => {
    set((state) => ({
      notebooks: state.notebooks.map((n) =>
        n.id === notebookId
          ? { ...n, sources: [...n.sources, source], updatedAt: new Date().toISOString() }
          : n
      ),
    }));
  },

  updateSource: (notebookId, sourceId, updates) => {
    set((state) => ({
      notebooks: state.notebooks.map((n) =>
        n.id === notebookId
          ? { ...n, sources: n.sources.map((s) => s.id === sourceId ? { ...s, ...updates } : s) }
          : n
      ),
    }));
  },

  removeSource: async (notebookId, sourceId) => {
    // loading 状态的 source 是本地 placeholder（ID 以 loading- 开头）
    if (sourceId.startsWith('loading-')) {
      // 查找 placeholder 关联的后端任务 ID，通知后端取消
      const notebook = get().notebooks.find(n => n.id === notebookId);
      const placeholder = notebook?.sources.find(s => s.id === sourceId);
      if (placeholder?.taskId) {
        try {
          await importApi.deleteImportTask(placeholder.taskId);
        } catch {
          // 任务可能已完成或不存在，忽略错误
        }
      }
      set((state) => ({
        notebooks: state.notebooks.map((n) =>
          n.id === notebookId
            ? { ...n, sources: n.sources.filter((s) => s.id !== sourceId) }
            : n
        ),
      }));
      return;
    }

    try {
      const res = await sourceApi.deleteSource(Number(notebookId), Number(sourceId));
      if (res.code === 0) {
        // 检查该 source 是否关联了正在运行的导入任务，有则取消
        const taskId = get().taskIdBySourceId[sourceId];
        if (taskId) {
          try {
            await importApi.deleteImportTask(taskId);
          } catch {
            // 任务可能已完成或不存在，忽略错误
          }
          // 清理映射
          set((state) => {
            const newMapping = { ...state.taskIdBySourceId };
            delete newMapping[sourceId];
            return { taskIdBySourceId: newMapping };
          });
        }

        set((state) => ({
          notebooks: state.notebooks.map((n) =>
            n.id === notebookId
              ? { ...n, sources: n.sources.filter((s) => s.id !== sourceId) }
              : n
          ),
        }));
      }
    } catch (err) {
      console.error('Failed to delete source:', err);
      throw err;
    }
  },

  batchRemoveSources: async (notebookId, sourceIds) => {
    try {
      // 找出 loading 状态的 placeholder，通知后端取消关联的导入任务
      const notebook = get().notebooks.find(n => n.id === notebookId);
      const loadingIds = sourceIds.filter(id => id.startsWith('loading-'));
      for (const loadingId of loadingIds) {
        const placeholder = notebook?.sources.find(s => s.id === loadingId);
        if (placeholder?.taskId) {
          try {
            await importApi.deleteImportTask(placeholder.taskId);
          } catch {
            // 任务可能已完成或不存在，忽略错误
          }
        }
      }

      // 找出关联了导入任务的真实 source，通知后端取消
      const taskIdsToCancel = new Set<string>();
      for (const sourceId of sourceIds) {
        const taskId = get().taskIdBySourceId[sourceId];
        if (taskId) taskIdsToCancel.add(taskId);
      }
      for (const taskId of taskIdsToCancel) {
        try {
          await importApi.deleteImportTask(taskId);
        } catch {
          // 忽略
        }
      }

      // 过滤出有效的数字 ID（排除 loading-xxx 等临时 ID）
      const validIds = sourceIds
        .filter(id => !id.startsWith('loading-'))
        .map(Number)
        .filter(id => !isNaN(id) && id > 0);

      // 如果有有效的 ID，调用批量删除 API
      if (validIds.length > 0) {
        const res = await sourceApi.batchDeleteSources(Number(notebookId), validIds);
        if (res.code !== 0) {
          throw new Error(res.message);
        }
      }

      // 清理映射
      set((state) => {
        const newMapping = { ...state.taskIdBySourceId };
        for (const sourceId of sourceIds) delete newMapping[sourceId];
        return { taskIdBySourceId: newMapping };
      });

      // 从本地状态中移除所有选中的 source（包括临时 placeholder）
      const idSet = new Set(sourceIds);
      set((state) => ({
        notebooks: state.notebooks.map((n) =>
          n.id === notebookId
            ? { ...n, sources: n.sources.filter((s) => !idSet.has(s.id)) }
            : n
        ),
      }));
    } catch (err) {
      console.error('Failed to batch delete sources:', err);
      throw err;
    }
  },

  toggleSourceSelection: (notebookId, sourceId) => {
    set((state) => ({
      notebooks: state.notebooks.map((n) =>
        n.id === notebookId
          ? { ...n, sources: n.sources.map((s) => s.id === sourceId ? { ...s, selected: !s.selected } : s) }
          : n
      ),
    }));
  },

  renameSource: async (notebookId, sourceId, name) => {
    try {
      const res = await sourceApi.renameSource(Number(notebookId), Number(sourceId), name);
      if (res.code === 0) {
        set((state) => ({
          notebooks: state.notebooks.map((n) =>
            n.id === notebookId
              ? { ...n, sources: n.sources.map((s) => s.id === sourceId ? { ...s, name } : s) }
              : n
          ),
        }));
      }
    } catch (err) {
      console.error('Failed to rename source:', err);
      throw err;
    }
  },

  selectAllSources: (notebookId) => {
    set((state) => ({
      notebooks: state.notebooks.map((n) =>
        n.id === notebookId
          ? { ...n, sources: n.sources.map((s) => ({ ...s, selected: true })) }
          : n
      ),
    }));
  },

  deselectAllSources: (notebookId) => {
    set((state) => ({
      notebooks: state.notebooks.map((n) =>
        n.id === notebookId
          ? { ...n, sources: n.sources.map((s) => ({ ...s, selected: false })) }
          : n
      ),
    }));
  },

  fetchSourceContent: async (notebookId, sourceId) => {
    const res = await sourceApi.getSourceContent(Number(notebookId), Number(sourceId));
    if (res.code === 0) return res.data.content;
    throw new Error(res.message);
  },

  fetchSourceOriginal: async (notebookId, sourceId) => {
    const res = await sourceApi.getSourceOriginal(Number(notebookId), Number(sourceId));
    if (res.code === 0) return { content: res.data.content, type: res.data.type };
    throw new Error(res.message);
  },

  getSourceDownloadURL: async (notebookId, sourceId) => {
    const res = await sourceApi.getSourceDownloadURL(Number(notebookId), Number(sourceId));
    if (res.code === 0) return res.data.url;
    throw new Error(res.message);
  },

  // ---- Import actions ----

  importFile: async (notebookId, file) => {
    // Optimistic: add a loading placeholder immediately
    const tempId = `loading-${Date.now()}-${Math.random().toString(36).slice(2, 6)}`;
    const placeholder: Source = {
      id: tempId,
      name: file.name,
      type: 'file',
      size: file.size,
      selected: true,
      status: 'loading',
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
    };
    get().addSource(notebookId, placeholder);

    try {
      const res = await importApi.importFile(Number(notebookId), file);
      if (res.code === 0) {
        const source = toSource(res.data);
        source.status = 'ready';
        // Replace placeholder with real source
        get().updateSource(notebookId, tempId, source);
        return source;
      }
      // API returned error code
      get().updateSource(notebookId, tempId, {
        status: 'error',
        errorMessage: res.message || '导入失败',
      });
      throw new Error(res.message);
    } catch (err: any) {
      // Mark placeholder as error (only if not already marked)
      const currentNotebook = get().notebooks.find(n => n.id === notebookId);
      const placeholderSource = currentNotebook?.sources.find(s => s.id === tempId);
      if (placeholderSource?.status !== 'error') {
        get().updateSource(notebookId, tempId, {
          status: 'error',
          errorMessage: getErrorMessage(err, '导入失败'),
        });
      }
      throw err;
    }
  },

  previewAudio: async (notebookId, file) => {
    // Optimistic: add a loading placeholder immediately
    const tempId = `loading-audio-${Date.now()}-${Math.random().toString(36).slice(2, 6)}`;
    const placeholder: Source = {
      id: tempId,
      name: file.name,
      type: 'audio',
      size: file.size,
      selected: true,
      status: 'loading',
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
    };
    get().addSource(notebookId, placeholder);

    try {
      const res = await importApi.previewAudio(Number(notebookId), file);
      if (res.code === 0) {
        // 上传成功，更新 placeholder 状态为 loading（等待转写完成）
        get().updateSource(notebookId, tempId, {
          previewId: res.data.preview_id,
        });
        // 后台轮询转写状态
        get().pollAudioPreview(res.data.preview_id).then((preview) => {
          get().updateSource(notebookId, tempId, {
            status: 'ready',
            content: preview.transcribed_text,
            previewId: preview.preview_id,
          });
        }).catch((err: any) => {
          get().updateSource(notebookId, tempId, {
            status: 'error',
            errorMessage: getErrorMessage(err, '音频转写失败'),
          });
        });
        return res.data;
      }
      // API returned error code
      get().updateSource(notebookId, tempId, {
        status: 'error',
        errorMessage: res.message || '音频转写失败',
      });
      throw new Error(res.message);
    } catch (err: any) {
      // Mark placeholder as error (only if not already marked)
      const currentNotebook = get().notebooks.find(n => n.id === notebookId);
      const placeholderSource = currentNotebook?.sources.find(s => s.id === tempId);
      if (placeholderSource?.status !== 'error') {
        get().updateSource(notebookId, tempId, {
          status: 'error',
          errorMessage: getErrorMessage(err, '音频转写失败'),
        });
      }
      throw err;
    }
  },

  pollAudioPreview: async (previewId) => {
    const maxAttempts = 120; // 最多轮询 120 次（约 4 分钟）
    const interval = 2000; // 每 2 秒轮询一次

    for (let i = 0; i < maxAttempts; i++) {
      const res = await importApi.getAudioPreviewStatus(previewId);
      if (res.code !== 0) {
        throw new Error(res.message || '查询转写状态失败');
      }
      const data = res.data;
      if (data.status === 'ready') {
        return data;
      }
      if (data.status === 'failed') {
        throw new Error(data.error_msg || '音频转写失败');
      }
      if (data.status === 'confirmed') {
        throw new Error('该音频已被确认过');
      }
      // pending 或 processing，继续轮询
      await new Promise((resolve) => setTimeout(resolve, interval));
    }
    throw new Error('音频转写超时，请稍后重试');
  },

  confirmAudio: async (previewId, notebookId, content) => {
    const res = await importApi.confirmAudio({
      preview_id: previewId,
      content: content || undefined,
      notebook_id: Number(notebookId),
    });
    if (res.code === 0) {
      const source = toSource(res.data);
      // Store the edited content locally so it can be viewed later
      source.content = content;
      // Remove the pending placeholder (has previewId) and add the confirmed source
      set((state) => ({
        notebooks: state.notebooks.map((n) =>
          n.id === notebookId
            ? { ...n, sources: [...n.sources.filter((s) => s.previewId !== previewId), source] }
            : n
        ),
      }));
      return source;
    }
    throw new Error(res.message);
  },

  getImportTask: async (taskId) => {
    const res = await importApi.getImportTask(taskId);
    if (res.code === 0) return res.data;
    throw new Error(res.message);
  },

  deleteImportTask: async (taskId) => {
    const res = await importApi.deleteImportTask(taskId);
    if (res.code !== 0) throw new Error(res.message);
  },

  // ---- Search actions ----

  searchSources: async (notebookId, query) => {
    const res = await searchApi.search(Number(notebookId), query);
    if (res.code === 0) return res.data;
    throw new Error(res.message);
  },

  searchSourcesStream: async (notebookId, query, onEvent, signal) => {
    await searchApi.searchStream(Number(notebookId), query, onEvent, signal);
  },

  importFromURL: async (notebookId, url) => {
    // 调用后端创建 pending 状态的 Source
    const res = await searchApi.importFromURL(Number(notebookId), url);
    if (res.code !== 0) throw new Error(res.message);

    const { task_id: taskId, source_id: sourceId } = res.data;

    // 记录 sourceID → taskID 映射，以便取消时使用
    set((state) => ({
      taskIdBySourceId: { ...state.taskIdBySourceId, [String(sourceId)]: taskId },
    }));

    // 刷新列表，后端创建的 pending source 会出现在列表中
    await get().fetchSources(notebookId);

    // 轮询该 source 的状态，直到处理完成
    const pollSource = async () => {
      const maxAttempts = 60;
      for (let i = 0; i < maxAttempts; i++) {
        await new Promise(r => setTimeout(r, 2000));
        try {
          await get().fetchSources(notebookId);
          const nb = get().notebooks.find(n => n.id === notebookId);
          const src = nb?.sources.find(s => s.id === String(sourceId));
          if (!src) return; // source 已被删除
          if (src.status !== 'loading') return; // 处理完成（ready 或 error）
        } catch {
          return;
        }
      }
    };
    pollSource();

    return { taskId, sourceId };
  },

  importSearchResults: async (notebookId, items) => {
    const res = await searchApi.importSearchResults(Number(notebookId), items);
    if (res.code !== 0) throw new Error(res.message);

    const { task_id: taskId, source_ids: sourceIds } = res.data;

    // 记录 sourceID → taskID 映射，以便删除 source 时取消任务
    const mapping: Record<string, string> = {};
    for (const sid of sourceIds) {
      mapping[String(sid)] = taskId;
    }
    set((state) => ({ taskIdBySourceId: { ...state.taskIdBySourceId, ...mapping } }));

    // 后端已创建 pending 状态的 Source，立即刷新列表显示它们
    await get().fetchSources(notebookId);

    // 轮询这些 Source 的状态，直到全部处理完成
    if (sourceIds.length > 0) {
      const pollSources = async () => {
        const maxAttempts = 120;
        for (let i = 0; i < maxAttempts; i++) {
          await new Promise(r => setTimeout(r, 2000));
          try {
            // 刷新 sources 列表获取最新状态
            await get().fetchSources(notebookId);

            // 检查这些 source 是否还有 pending/processing 状态的
            const notebook = get().notebooks.find(n => n.id === notebookId);
            if (!notebook) return;
            const pendingCount = notebook.sources.filter(
              s => sourceIds.includes(Number(s.id)) && (s.status === 'loading')
            ).length;
            if (pendingCount === 0) {
              // 全部处理完成，清理映射
              set((state) => {
                const newMapping = { ...state.taskIdBySourceId };
                for (const sid of sourceIds) delete newMapping[String(sid)];
                return { taskIdBySourceId: newMapping };
              });
              return;
            }
          } catch {
            return;
          }
        }
      };
      pollSources();
    }

    return { taskId, sourceIds };
  },

  // ---- Conversation actions (API-backed) ----

  fetchConversations: async (notebookId) => {
    try {
      const res = await chatApi.listConversations(Number(notebookId));
      if (res.code === 0) {
        const conversations: Conversation[] = res.data.map((c) => ({
          id: String(c.id),
          title: c.title,
          messages: [],
          createdAt: c.created_at,
          updatedAt: c.updated_at,
        }));
        set((state) => ({
          notebooks: state.notebooks.map((n) =>
            n.id === notebookId ? { ...n, conversations } : n
          ),
          // 如果当前没有选中对话，自动选中第一个
          currentConversationId: get().currentConversationId || conversations[0]?.id || null,
        }));
      }
    } catch (err) {
      console.error('Failed to fetch conversations:', err);
    }
  },

  createConversation: async (notebookId) => {
    try {
      const res = await chatApi.createConversation({
        notebook_id: Number(notebookId),
      });
      if (res.code === 0) {
        const newConv: Conversation = {
          id: String(res.data.id),
          title: '新对话',
          messages: [],
          createdAt: new Date().toISOString(),
          updatedAt: new Date().toISOString(),
        };
        set((state) => ({
          notebooks: state.notebooks.map((n) =>
            n.id === notebookId
              ? { ...n, conversations: [newConv, ...n.conversations] }
              : n
          ),
          currentConversationId: newConv.id,
        }));
        return newConv.id;
      }
    } catch (err) {
      console.error('Failed to create conversation:', err);
    }
    return null;
  },

  setCurrentConversation: (id) => set({ currentConversationId: id }),

  deleteConversation: async (notebookId, conversationId) => {
    try {
      const res = await chatApi.deleteConversation(Number(conversationId));
      if (res.code === 0) {
        set((state) => {
          const notebook = state.notebooks.find((n) => n.id === notebookId);
          if (!notebook) return state;
          const filtered = notebook.conversations.filter((c) => c.id !== conversationId);
          return {
            notebooks: state.notebooks.map((n) =>
              n.id === notebookId ? { ...n, conversations: filtered } : n
            ),
            currentConversationId:
              state.currentConversationId === conversationId
                ? (filtered[0]?.id ?? null)
                : state.currentConversationId,
          };
        });
      }
    } catch (err) {
      console.error('Failed to delete conversation:', err);
    }
  },

  addMessage: (notebookId, conversationId, message) => {
    set((state) => ({
      notebooks: state.notebooks.map((n) =>
        n.id === notebookId
          ? {
              ...n,
              conversations: n.conversations.map((c) =>
                c.id === conversationId
                  ? {
                      ...c,
                      messages: [...c.messages, message],
                      updatedAt: new Date().toISOString(),
                      title: c.messages.length === 0 && message.role === 'user'
                        ? message.content.slice(0, 20)
                        : c.title,
                    }
                  : c
              ),
            }
          : n
      ),
    }));
  },

  addNote: (notebookId, note) => {
    set((state) => ({
      notebooks: state.notebooks.map((n) =>
        n.id === notebookId
          ? { ...n, notes: [note, ...n.notes], updatedAt: new Date().toISOString() }
          : n
      ),
    }));
  },

  deleteNote: (notebookId, noteId) => {
    set((state) => ({
      notebooks: state.notebooks.map((n) =>
        n.id === notebookId
          ? { ...n, notes: n.notes.filter((note) => note.id !== noteId) }
          : n
      ),
    }));
  },

  renameNote: (notebookId, noteId, title) => {
    set((state) => ({
      notebooks: state.notebooks.map((n) =>
        n.id === notebookId
          ? { ...n, notes: n.notes.map((note) => note.id === noteId ? { ...note, title } : note) }
          : n
      ),
    }));
  },

  updateNoteContent: (notebookId, noteId, content) => {
    set((state) => ({
      notebooks: state.notebooks.map((n) =>
        n.id === notebookId
          ? { ...n, notes: n.notes.map((note) => note.id === noteId ? { ...note, content, updatedAt: new Date().toISOString() } : note) }
          : n
      ),
    }));
  },

  toggleNoteSource: (notebookId, noteId) => {
    set((state) => ({
      notebooks: state.notebooks.map((n) =>
        n.id === notebookId
          ? { ...n, notes: n.notes.map((note) => note.id === noteId ? { ...note, isSource: !note.isSource } : note) }
          : n
      ),
    }));
  },
}));
