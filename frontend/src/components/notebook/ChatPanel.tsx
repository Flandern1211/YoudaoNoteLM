import { useState, useRef, useEffect } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import {
  Send, Plus, MessageSquare, Trash2, Save, Loader2,
  ChevronDown, Sparkles, Edit3
} from 'lucide-react';
import { useNotebookStore } from '../../stores/useNotebookStore';
import * as chatApi from '../../api/chat';
import { cn } from '../../utils/cn';
import type { ChatMessage, NoteType } from '../../types';

export default function ChatPanel() {
  const {
    currentNotebookId, getCurrentNotebook, getCurrentConversation,
    createConversation, setCurrentConversation, deleteConversation, addMessage, addNote,
    fetchConversations
  } = useNotebookStore();

  const notebook = getCurrentNotebook();
  const conversation = getCurrentConversation();

  const [input, setInput] = useState('');
  const [isStreaming, setIsStreaming] = useState(false);
  const [showConvList, setShowConvList] = useState(false);
  const [streamingText, setStreamingText] = useState('');
  const [editingConvId, setEditingConvId] = useState<string | null>(null);
  const [editConvTitle, setEditConvTitle] = useState('');
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const abortControllerRef = useRef<AbortController | null>(null);
  const streamingContentRef = useRef('');

  // 初始加载对话列表
  useEffect(() => {
    if (currentNotebookId) {
      fetchConversations(currentNotebookId);
    }
  }, [currentNotebookId, fetchConversations]);

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [conversation?.messages, streamingText]);

  if (!notebook || !currentNotebookId) return null;

  const selectedSources = notebook.sources.filter((s) => s.selected);

  const handleSend = async () => {
    if (!input.trim() || isStreaming) return;

    const userContent = input.trim();

    // 确保有对话
    let convId = conversation?.id;
    if (!convId) {
      const newId = await createConversation(currentNotebookId);
      if (!newId) return;
      convId = newId;
    }

    // 添加用户消息到 UI
    const userMsg: ChatMessage = {
      id: `msg-${Date.now()}`,
      role: 'user',
      content: userContent,
      timestamp: new Date().toISOString(),
    };
    addMessage(currentNotebookId, convId, userMsg);
    setInput('');
    setIsStreaming(true);
    setStreamingText('');

    // 创建 AbortController
    const controller = new AbortController();
    abortControllerRef.current = controller;
    streamingContentRef.current = '';

    try {
      await chatApi.sendMessage(
        Number(convId),
        {
          content: userContent,
          source_ids: selectedSources.map((s) => Number(s.id)),
        },
        (event) => {
          switch (event.type) {
            case 'token':
              if (event.content) {
                streamingContentRef.current += event.content;
                setStreamingText(streamingContentRef.current);
              }
              break;
            case 'error':
              if (event.content) {
                streamingContentRef.current += `\n\n❌ ${event.content}`;
                setStreamingText(streamingContentRef.current);
              }
              break;
            case 'done':
              break;
          }
        },
        controller.signal,
      );
    } catch (err: any) {
      if (err.name !== 'AbortError') {
        console.error('Chat stream error:', err);
        if (!streamingContentRef.current) {
          streamingContentRef.current = '❌ 发送失败，请重试';
          setStreamingText(streamingContentRef.current);
        }
      }
    } finally {
      // 将流式文本转为正式消息
      const finalText = streamingContentRef.current;
      if (finalText) {
        const assistantMsg: ChatMessage = {
          id: `msg-${Date.now() + 1}`,
          role: 'assistant',
          content: finalText,
          timestamp: new Date().toISOString(),
          citations: selectedSources.length > 0 ? selectedSources.map((s) => s.id) : undefined,
        };
        addMessage(currentNotebookId, convId, assistantMsg);
      }

      setIsStreaming(false);
      setStreamingText('');
      streamingContentRef.current = '';
      abortControllerRef.current = null;
    }
  };

  const handleStop = async () => {
    if (abortControllerRef.current) {
      abortControllerRef.current.abort();
    }
    if (conversation?.id) {
      try {
        await chatApi.stopGeneration(Number(conversation.id));
      } catch {
        // 忽略
      }
    }
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleSend();
    }
  };

  const handleSaveAsNote = (content: string) => {
    const note = {
      id: `note-${Date.now()}`,
      title: content.slice(0, 20).replace(/[#*\n]/g, ''),
      type: 'note' as NoteType,
      content,
      isSource: false,
      notebookId: currentNotebookId,
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
    };
    addNote(currentNotebookId, note);
  };

  const handleStartRenameConv = (id: string, title: string) => {
    setEditingConvId(id);
    setEditConvTitle(title);
  };

  const handleFinishRenameConv = async () => {
    if (editingConvId && editConvTitle.trim()) {
      try {
        await chatApi.updateConversation(Number(editingConvId), editConvTitle.trim());
        // 更新本地状态
        useNotebookStore.setState((s) => ({
          notebooks: s.notebooks.map(n =>
            n.id === currentNotebookId
              ? {
                  ...n,
                  conversations: n.conversations.map(c =>
                    c.id === editingConvId ? { ...c, title: editConvTitle.trim() } : c
                  ),
                }
              : n
          ),
        }));
      } catch (err) {
        console.error('Failed to rename conversation:', err);
      }
    }
    setEditingConvId(null);
  };

  return (
    <div className="h-full flex flex-col">
      {/* Header with conversation switcher */}
      <div className="flex items-center justify-between px-4 py-3 border-b border-border flex-shrink-0">
        <div className="relative">
          <button
            onClick={() => setShowConvList(!showConvList)}
            className="flex items-center gap-2 text-sm font-medium text-text-primary hover:text-accent transition-colors cursor-pointer"
          >
            <MessageSquare size={15} />
            <span className="max-w-[200px] truncate">{conversation?.title || '新对话'}</span>
            <ChevronDown size={13} className={cn('transition-transform', showConvList && 'rotate-180')} />
          </button>

          <AnimatePresence>
            {showConvList && (
              <>
                <div className="fixed inset-0 z-40" onClick={() => setShowConvList(false)} />
                <motion.div
                  initial={{ opacity: 0, y: -4 }}
                  animate={{ opacity: 1, y: 0 }}
                  exit={{ opacity: 0, y: -4 }}
                  className="absolute left-0 top-full mt-2 w-72 bg-bg-card border border-border-light rounded-xl shadow-xl z-50 max-h-80 overflow-y-auto"
                >
                  <div className="p-2">
                    <button
                      onClick={() => {
                        createConversation(currentNotebookId);
                        setShowConvList(false);
                      }}
                      className="w-full flex items-center gap-2 px-3 py-2 rounded-lg text-xs text-accent hover:bg-accent-glow transition-colors cursor-pointer"
                    >
                      <Plus size={13} /> 新建对话
                    </button>
                  </div>
                  <div className="border-t border-border px-2 py-1">
                    {notebook.conversations.map((conv) => (
                      <div
                        key={conv.id}
                        className={cn(
                          'group flex items-center gap-2 px-3 py-2 rounded-lg cursor-pointer transition-all',
                          conversation?.id === conv.id ? 'bg-accent/10 text-accent' : 'text-text-secondary hover:bg-bg-hover'
                        )}
                        onClick={() => {
                          setCurrentConversation(conv.id);
                          setShowConvList(false);
                        }}
                      >
                        <MessageSquare size={12} className="flex-shrink-0" />
                        {editingConvId === conv.id ? (
                          <input
                            autoFocus
                            value={editConvTitle}
                            onChange={(e) => setEditConvTitle(e.target.value)}
                            onBlur={handleFinishRenameConv}
                            onKeyDown={(e) => {
                              if (e.key === 'Enter') handleFinishRenameConv();
                              if (e.key === 'Escape') setEditingConvId(null);
                            }}
                            onClick={(e) => e.stopPropagation()}
                            className="flex-1 text-xs bg-transparent outline-none border-b border-accent"
                          />
                        ) : (
                          <span className="flex-1 text-xs truncate">{conv.title}</span>
                        )}
                        <button
                          onClick={(e) => {
                            e.stopPropagation();
                            handleStartRenameConv(conv.id, conv.title);
                          }}
                          className="opacity-0 group-hover:opacity-100 p-0.5 rounded hover:bg-bg-active transition-all cursor-pointer"
                        >
                          <Edit3 size={11} />
                        </button>
                        <button
                          onClick={(e) => {
                            e.stopPropagation();
                            deleteConversation(currentNotebookId, conv.id);
                          }}
                          className="opacity-0 group-hover:opacity-100 p-0.5 rounded hover:bg-error/10 transition-all cursor-pointer"
                        >
                          <Trash2 size={11} className="text-error" />
                        </button>
                      </div>
                    ))}
                  </div>
                </motion.div>
              </>
            )}
          </AnimatePresence>
        </div>

        {/* Right side spacer for balance */}
        <div />
      </div>

      {/* Messages */}
      <div className="flex-1 overflow-y-auto px-4 py-4 space-y-4">
        {(!conversation || conversation.messages.length === 0) && !isStreaming && (
          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            className="flex flex-col items-center justify-center h-full text-center"
          >
            <div className="w-16 h-16 rounded-2xl bg-gradient-to-br from-accent/20 to-teal/20 flex items-center justify-center mb-4">
              <Sparkles size={28} className="text-accent" />
            </div>
            <h3 className="text-base font-semibold text-text-primary mb-2">开始对话</h3>
            <p className="text-sm text-text-secondary max-w-xs">
              {selectedSources.length > 0
                ? `已选中 ${selectedSources.length} 份资料，输入问题开始对话`
                : '输入问题开始对话，或在左侧选择资料来源获得更精准的回答'}
            </p>
            <div className="flex flex-wrap gap-2 mt-4 justify-center">
              {['帮我总结核心观点', '生成思维导图', '出 10 道测验题'].map((q) => (
                <button
                  key={q}
                  onClick={() => setInput(q)}
                  className="px-3 py-1.5 rounded-full text-xs bg-bg-card border border-border-light text-text-secondary hover:text-accent hover:border-accent/30 transition-all cursor-pointer"
                >
                  {q}
                </button>
              ))}
            </div>
          </motion.div>
        )}

        {conversation?.messages.map((msg) => (
          <motion.div
            key={msg.id}
            initial={{ opacity: 0, y: 8 }}
            animate={{ opacity: 1, y: 0 }}
            className={cn('flex gap-3', msg.role === 'user' ? 'justify-end' : 'justify-start')}
          >
            {msg.role === 'assistant' && (
              <div className="w-7 h-7 rounded-lg bg-gradient-to-br from-accent to-teal flex items-center justify-center flex-shrink-0 mt-0.5">
                <Sparkles size={13} className="text-white" />
              </div>
            )}
            <div
              className={cn(
                'max-w-[80%] rounded-2xl px-4 py-3 text-sm',
                msg.role === 'user'
                  ? 'bg-accent text-white rounded-br-md'
                  : 'bg-bg-card border border-border-light rounded-bl-md'
              )}
            >
              <div className="whitespace-pre-wrap leading-relaxed">
                {msg.content.split('\n').map((line, i) => {
                  if (line.startsWith('**') && line.endsWith('**')) {
                    return <p key={i} className="font-semibold my-1">{line.replace(/\*\*/g, '')}</p>;
                  }
                  if (line.startsWith('> ')) {
                    return <p key={i} className="border-l-2 border-accent/40 pl-2 my-1 text-text-muted italic text-xs">{line.slice(2)}</p>;
                  }
                  if (line.startsWith('- ')) {
                    return <p key={i} className="ml-3 my-0.5 before:content-['•'] before:mr-2 before:text-accent">{line.slice(2)}</p>;
                  }
                  if (line.match(/^\d+\.\s/)) {
                    return <p key={i} className="ml-3 my-0.5">{line}</p>;
                  }
                  if (line.startsWith('```')) return <div key={i} className="my-1 h-px bg-border-light" />;
                  return <p key={i} className="my-0.5">{line}</p>;
                })}
              </div>

              {msg.role === 'assistant' && (
                <div className="flex items-center gap-2 mt-3 pt-2 border-t border-border">
                  <button
                    onClick={() => handleSaveAsNote(msg.content)}
                    className="flex items-center gap-1 text-xs text-text-muted hover:text-accent transition-colors cursor-pointer"
                  >
                    <Save size={11} /> 保存为笔记
                  </button>
                  {msg.citations && msg.citations.length > 0 && (
                    <span className="text-xs text-text-muted">
                      引用 {msg.citations.length} 份资料
                    </span>
                  )}
                </div>
              )}
            </div>
            {msg.role === 'user' && (
              <div className="w-7 h-7 rounded-lg bg-accent/20 flex items-center justify-center flex-shrink-0 mt-0.5">
                <span className="text-xs font-bold text-accent">我</span>
              </div>
            )}
          </motion.div>
        ))}

        {/* Streaming message */}
        {isStreaming && (
          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            className="flex gap-3"
          >
            <div className="w-7 h-7 rounded-lg bg-gradient-to-br from-accent to-teal flex items-center justify-center flex-shrink-0 mt-0.5">
              <Sparkles size={13} className="text-white" />
            </div>
            <div className="max-w-[80%] rounded-2xl rounded-bl-md px-4 py-3 bg-bg-card border border-border-light">
              {streamingText ? (
                <div className="whitespace-pre-wrap text-sm leading-relaxed">
                  {streamingText}
                  <span className="inline-block w-0.5 h-4 bg-accent ml-0.5 animate-pulse" />
                </div>
              ) : (
                <div className="flex items-center gap-2 py-1">
                  <Loader2 size={14} className="animate-spin text-accent" />
                  <span className="text-sm text-text-muted">正在思考...</span>
                </div>
              )}
            </div>
          </motion.div>
        )}

        <div ref={messagesEndRef} />
      </div>

      {/* Input area */}
      <div className="px-4 pb-4 flex-shrink-0">
        <div className="chat-input-container relative rounded-2xl border border-border-light bg-bg-card transition-colors">
          <textarea
            value={input}
            onChange={(e) => setInput(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder='输入问题，或说"帮我生成思维导图"...'
            rows={2}
            className="w-full bg-transparent text-sm text-text-primary placeholder:text-text-muted px-4 py-3 resize-none outline-none"
          />
          <div className="flex items-center justify-between px-3 pb-2">
            <div className="flex items-center gap-1.5">
              {selectedSources.length > 0 && (
                <span className="text-[10px] text-accent bg-accent-glow px-1.5 py-0.5 rounded">
                  基于 {selectedSources.length} 份资料
                </span>
              )}
            </div>
            <div className="flex items-center gap-1.5">
              {isStreaming && (
                <button
                  onClick={handleStop}
                  className="px-2 py-1 rounded-lg text-xs bg-error/10 text-error hover:bg-error/20 transition-colors cursor-pointer"
                >
                  停止
                </button>
              )}
              <button
                onClick={handleSend}
                disabled={!input.trim() || isStreaming}
                className={cn(
                  'p-2 rounded-lg transition-all cursor-pointer',
                  input.trim() && !isStreaming
                    ? 'bg-accent text-white hover:bg-accent-light shadow-md shadow-accent/30'
                    : 'bg-bg-hover text-text-muted cursor-not-allowed'
                )}
              >
                <Send size={16} />
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
