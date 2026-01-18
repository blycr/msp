export interface Subtitle {
  id: string;
  label: string;
  lang: string;
  src: string;
  default?: boolean;
}

export interface MediaFile {
  id: string;
  name: string;
  ext: string;
  kind: 'video' | 'audio' | 'image' | 'other'; // Inferring 'other' from usage
  shareLabel: string;
  size: number;
  modTime: number;
  subtitles?: Subtitle[];
  coverId?: string;
  lyricsId?: string;
}

export interface Breadcrumb {
  name: string;
  path: string;
  active: boolean;
}

export interface ShareConfig {
  label: string;
  path: string;
}

export interface FeaturesConfig {
  speed: boolean;
  speedOptions: number[];
  quality: boolean;
  captions: boolean;
  playlist: boolean;
}

export interface UIConfig {
  defaultTab?: string;
  showOthers?: boolean;
}

export interface PlaybackAudioConfig {
  enabled?: boolean;
  shuffle?: boolean;
  remember?: boolean;
  scope?: string;
  transcode?: boolean;
}

export interface PlaybackVideoConfig {
  enabled?: boolean;
  scope?: string;
  transcode?: boolean;
  resume?: boolean;
}

export interface PlaybackImageConfig {
  enabled?: boolean;
  scope?: string;
}

export interface PlaybackConfig {
  audio: PlaybackAudioConfig;
  video: PlaybackVideoConfig;
  image: PlaybackImageConfig;
}

export interface BlacklistConfig {
  extensions: string[];
  filenames: string[];
  folders: string[];
  sizeRule: string;
}

export interface SecurityConfig {
  ipWhitelist: string[];
  ipBlacklist: string[];
  pinEnabled: boolean;
  pin: string;
}

export interface ServerConfig {
  port: number;
  shares: ShareConfig[];
  features: FeaturesConfig;
  ui: UIConfig;
  playback: PlaybackConfig;
  blacklist: BlacklistConfig;
  security: SecurityConfig;
  logLevel: string;
  logFile: string;
  maxItems: number;
}

export interface ConfigResponse {
  config: ServerConfig;
  lanIPs: string[];
  urls: string[];
  nowUnix: number;
  error?: ApiError;
}

export interface MediaResponse {
  shares: ShareConfig[];
  videos: MediaFile[];
  audios: MediaFile[];
  images: MediaFile[];
  others: MediaFile[];
  videosTotal?: number;
  audiosTotal?: number;
  imagesTotal?: number;
  othersTotal?: number;
  limited?: boolean;
  scanning?: boolean;
}

export interface ApiError {
  message: string;
}

export interface ProbeResponse {
  container: string;
  video?: string;
  audio?: string;
  subtitles?: Subtitle[];
  error?: ApiError;
}

export interface PrefsResponse {
  prefs: Record<string, string>;
  error?: ApiError;
}

export interface LogRequest {
  level: string;
  msg: string;
}

export interface SharesOpRequest {
  op: 'add' | 'remove';
  path: string;
  label: string;
}
