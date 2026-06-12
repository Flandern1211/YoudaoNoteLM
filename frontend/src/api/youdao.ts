import client from './client';
import type { ApiResponse } from './auth';

// ============ Types ============

export interface YoudaoNote {
  note_id: string;
  title: string;
  content: string;
  created_at: string;
  updated_at: string;
}

export interface YoudaoNotebook {
  notebook_id: string;
  name: string;
  notes: YoudaoNote[];
}

export interface YoudaoBindStatus {
  bound: boolean;
  status?: string;
  email?: string;
  bind_time?: string;
}

// ============ API Functions ============

// 1. List Youdao notebooks
export async function listNotebooks(): Promise<ApiResponse<YoudaoNotebook[]>> {
  const res = await client.get<ApiResponse<YoudaoNotebook[]>>('/youdao/notebooks');
  return res.data;
}

// 2. Import notes from Youdao
export async function importNotesBatch(
  noteIds: string[],
  notebookId: number,
  noteNames?: Record<string, string>
): Promise<ApiResponse<{ task_id: string; source_ids: number[] }>> {
  const res = await client.post<ApiResponse<{ task_id: string; source_ids: number[] }>>('/youdao/import', {
    note_ids: noteIds,
    notebook_id: notebookId,
    note_names: noteNames,
  });
  return res.data;
}

// 3. Get bind status
export async function getBindStatus(): Promise<ApiResponse<YoudaoBindStatus>> {
  const res = await client.get<ApiResponse<YoudaoBindStatus>>('/youdao/bind-status');
  return res.data;
}

// 4. Bind API key
export async function bindApiKey(apiKey: string): Promise<ApiResponse> {
  const res = await client.post<ApiResponse>('/youdao/bind', { api_key: apiKey });
  return res.data;
}

// 5. Unbind
export async function unbind(): Promise<ApiResponse> {
  const res = await client.post<ApiResponse>('/youdao/unbind');
  return res.data;
}
