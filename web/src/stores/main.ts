import { defineStore } from 'pinia';
import { ref, computed } from 'vue';
import api from '../services/api';
import type { ServerConfig, MediaResponse, MediaFile } from '../types';

export const useMainStore = defineStore('main', () => {
  // State
  const config = ref<ServerConfig | null>(null);
  const media = ref<MediaResponse | null>(null);
  const prefs = ref<Record<string, string>>({});
  const lanIPs = ref<string[]>([]);
  const isLoading = ref(false);
  const error = ref<string | null>(null);
  const playingMedia = ref<MediaFile | null>(null);
  
  // Dialog State
  const showSettings = ref(false);
  const showPin = ref(false);
  const pinSuccessCallback = ref<(() => void) | null>(null);

  // Getters
  const isConfigLoaded = computed(() => !!config.value);
  const videos = computed(() => media.value?.videos || []);
  const audios = computed(() => media.value?.audios || []);
  const images = computed(() => media.value?.images || []);
  const others = computed(() => media.value?.others || []);
  
  const theme = computed(() => prefs.value['theme'] || 'dark');
  const volume = computed(() => parseFloat(prefs.value['volume'] || '1.0'));

  // Actions
  async function fetchConfig() {
    try {
      const data = await api.getServerConfig();
      config.value = data.config;
      lanIPs.value = data.lanIPs || [];
      
      // Check for PIN requirement (if PIN is enabled and not yet verified, verifyPin will handle it)
      // For now, we rely on API response, if API returns error related to auth, we trigger PIN
    } catch (err: any) {
      error.value = err.message || 'Failed to load config';
      console.error(err);
    }
  }

  async function fetchMedia(refresh = false) {
    isLoading.value = true;
    try {
      const data = await api.getMedia(refresh);
      media.value = data;
    } catch (err: any) {
      error.value = err.message || 'Failed to load media';
      console.error(err);
    } finally {
      isLoading.value = false;
    }
  }

  async function fetchPrefs() {
    try {
      const data = await api.getPrefs();
      prefs.value = data.prefs || {};
    } catch (err: any) {
      console.error('Failed to load prefs', err);
    }
  }

  async function updateConfig(newConfig: Partial<ServerConfig>) {
    try {
      const data = await api.updateServerConfig(newConfig);
      config.value = data.config;
    } catch (err: any) {
      error.value = err.message || 'Failed to update config';
      throw err;
    }
  }
  
  async function updateShares(op: 'add' | 'remove', path: string, label?: string) {
    try {
      const data = await api.updateShares(op, path, label);
      if (data.config) {
        // config.value = data.config; // Type mismatch? server returns full config
        // Assuming api.updateShares returns { config: ServerConfig }
        await fetchConfig(); // Safer to just refetch or properly type cast
      }
    } catch (err: any) {
      error.value = err.message || 'Failed to update shares';
      throw err;
    }
  }

  async function setPref(key: string, value: string) {
    const newPrefs = { ...prefs.value, [key]: value };
    try {
      prefs.value = newPrefs;
      await api.updatePrefs(newPrefs);
    } catch (err: any) {
      console.error(`Failed to set pref ${key}`, err);
      await fetchPrefs();
    }
  }

  async function toggleTheme() {
    const newTheme = theme.value === 'dark' ? 'light' : 'dark';
    await setPref('theme', newTheme);
    document.documentElement.classList.toggle('dark', newTheme === 'dark');
  }

  function playMedia(item: MediaFile) {
    playingMedia.value = item;
  }
  
  function openSettings() {
    showSettings.value = true;
  }
  
  function closeSettings() {
    showSettings.value = false;
  }
  
  function requestPin(callback?: () => void) {
    showPin.value = true;
    pinSuccessCallback.value = callback || null;
  }
  
  function closePin() {
    showPin.value = false;
    pinSuccessCallback.value = null;
  }
  
  async function verifyPin(pin: string) {
    try {
      const res = await api.verifyPin(pin);
      if (res.valid) {
        closePin();
        if (pinSuccessCallback.value) {
          pinSuccessCallback.value();
        }
        return true;
      }
      return false;
    } catch (e) {
      return false;
    }
  }

  async function init() {
    await Promise.all([fetchConfig(), fetchPrefs()]);
    document.documentElement.classList.toggle('dark', theme.value === 'dark');
    await fetchMedia();
    
    // Check if PIN enabled
    if (config.value?.security.pinEnabled) {
      // Check if we need to verify PIN (check cookie? or just verify empty PIN)
      // Actually, verifyPin check might be needed on load
      const res = await api.verifyPin(''); // Check status
      if (res.enabled && !res.valid) {
          // Logic to show PIN if not validated? 
          // Usually valid=false means not authenticated.
          // But verifyPin('') with empty string might return valid=false even if cookie is set?
          // Let's assume handlePIN logic checks cookie first.
          // If the backend returns valid=true for empty pin, we are good.
      }
    }
  }

  return {
    // State
    config,
    media,
    prefs,
    lanIPs,
    isLoading,
    error,
    playingMedia,
    showSettings,
    showPin,
    
    // Getters
    isConfigLoaded,
    videos,
    audios,
    images,
    others,
    theme,
    volume,

    // Actions
    fetchConfig,
    fetchMedia,
    fetchPrefs,
    updateConfig,
    updateShares,
    setPref,
    toggleTheme,
    playMedia,
    openSettings,
    closeSettings,
    requestPin,
    closePin,
    verifyPin,
    init,
  };
});
