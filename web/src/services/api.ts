import axios from 'axios';
import type {
  ConfigResponse,
  ServerConfig,
  SharesOpRequest,
  // SharesOpResponse, // Not exported in types/index.ts yet, I should add it or inline it
  PrefsResponse,
  LogRequest,
  MediaResponse,
  ProbeResponse,
} from '../types';

// Add missing types that were not in internal/types/types.go but used in handlers
export interface SharesOpResponse {
  config: any; // Ideally typed as ServerConfig or similar
  error?: { message: string };
}

export interface ProgressResponse {
  time: number;
}

export interface PinResponse {
  valid: boolean;
  enabled: boolean;
  error?: string;
}

const api = axios.create({
  baseURL: '/api',
});

export const getServerConfig = async (): Promise<ConfigResponse> => {
  const { data } = await api.get<ConfigResponse>('/config');
  return data;
};

export const updateServerConfig = async (config: Partial<ServerConfig>): Promise<ConfigResponse> => {
  const { data } = await api.post<ConfigResponse>('/config', config);
  return data;
};

export const updateShares = async (op: 'add' | 'remove', path: string, label?: string): Promise<SharesOpResponse> => {
  const payload: SharesOpRequest = { op, path, label: label || '' };
  const { data } = await api.post<SharesOpResponse>('/shares', payload);
  return data;
};

export const getLanIps = async (): Promise<{ lanIPs: string[] }> => {
  const { data } = await api.get<{ lanIPs: string[] }>('/ip');
  return data;
};

export const getPrefs = async (): Promise<PrefsResponse> => {
  const { data } = await api.get<PrefsResponse>('/prefs');
  return data;
};

export const updatePrefs = async (prefs: Record<string, string>): Promise<PrefsResponse> => {
  const { data } = await api.post<PrefsResponse>('/prefs', { prefs });
  return data;
};

export const getProgress = async (id: string): Promise<number> => {
  const { data } = await api.get<ProgressResponse>('/progress', { params: { id } });
  return data.time;
};

export const setProgress = async (id: string, time: number): Promise<void> => {
  await api.post('/progress', { id, time });
};

export const sendLog = async (level: string, msg: string): Promise<void> => {
  const payload: LogRequest = { level, msg };
  await api.post('/log', payload);
};

export const verifyPin = async (pin: string): Promise<PinResponse> => {
  const { data } = await api.post<PinResponse>('/pin', { pin });
  return data;
};

export const getMedia = async (refresh = false, limit = 0): Promise<MediaResponse> => {
  const { data } = await api.get<MediaResponse>('/media', {
    params: {
      refresh: refresh ? '1' : '0',
      limit: limit > 0 ? limit.toString() : undefined,
    },
  });
  return data;
};

export const getProbe = async (id: string): Promise<ProbeResponse> => {
  const { data } = await api.get<ProbeResponse>('/probe', { params: { id } });
  return data;
};

export const getStreamUrl = (id: string, transcode = false, start = 0): string => {
  const params = new URLSearchParams();
  params.append('id', id);
  if (transcode) {
    params.append('transcode', '1');
  }
  if (start > 0) {
    params.append('start', start.toString());
  }
  return `/api/stream?${params.toString()}`;
};

export const getSubtitleUrl = (id: string): string => {
  return `/api/subtitle?id=${encodeURIComponent(id)}`;
};

export default {
  getServerConfig,
  updateServerConfig,
  updateShares,
  getLanIps,
  getPrefs,
  updatePrefs,
  getProgress,
  setProgress,
  sendLog,
  verifyPin,
  getMedia,
  getProbe,
  getStreamUrl,
  getSubtitleUrl,
};
