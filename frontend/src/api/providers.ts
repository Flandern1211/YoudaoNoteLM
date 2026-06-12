import client from './client';
import type { ApiResponse } from './auth';

// ============ Types ============

export interface ProviderInfo {
  name: string;
  provider: string;
  display_name: string;
  description: string;
  type: string;
  key_labels?: Record<string, string>;
  required_keys?: string[];
  optional_keys?: string[];
  config_schema?: Record<string, any>;
}

// ============ API Functions ============

// 1. Get all providers
export async function getProviders(): Promise<ApiResponse<ProviderInfo[]>> {
  const res = await client.get<ApiResponse<ProviderInfo[]>>('/providers');
  return res.data;
}

// 2. Get providers by type
export async function getProvidersByType(type: string): Promise<ApiResponse<ProviderInfo[]>> {
  const res = await client.get<ApiResponse<ProviderInfo[]>>(`/providers?type=${type}`);
  return res.data;
}

// 3. Get active config for a provider type
export async function getActiveConfig(type: string): Promise<ApiResponse<{ source: string; provider: string; display_name: string } | null>> {
  const res = await client.get<ApiResponse<{ source: string; provider: string; display_name: string } | null>>(`/providers/active?type=${type}`);
  return res.data;
}
