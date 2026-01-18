<script setup lang="ts">
import { ref, watch, onUnmounted, computed, nextTick } from 'vue';
import { useMainStore } from '@/stores/main';
import api from '@/services/api';
import Plyr from 'plyr';
import 'plyr/dist/plyr.css';
import { useDebounceFn } from '@vueuse/core';

const store = useMainStore();
const videoEl = ref<HTMLVideoElement | null>(null);
const audioEl = ref<HTMLAudioElement | null>(null);
const plyrInstance = ref<Plyr | null>(null);

const currentMedia = computed(() => store.playingMedia);

// Determine media type for conditional rendering
const isVideo = computed(() => currentMedia.value?.kind === 'video');
const isAudio = computed(() => currentMedia.value?.kind === 'audio');
const isImage = computed(() => currentMedia.value?.kind === 'image');

// Sources
const streamUrl = computed(() => 
  currentMedia.value ? api.getStreamUrl(currentMedia.value.id) : ''
);

// Progress Saving
const saveProgress = useDebounceFn(async (time: number) => {
  if (currentMedia.value) {
    await api.setProgress(currentMedia.value.id, time);
  }
}, 2000);

const initPlayer = async () => {
  // Cleanup previous
  if (plyrInstance.value) {
    plyrInstance.value.destroy();
    plyrInstance.value = null;
  }

  await nextTick(); // Wait for DOM update

  if (isVideo.value && videoEl.value) {
    plyrInstance.value = new Plyr(videoEl.value, {
      autoplay: true,
      keyboard: { global: true },
      controls: [
        'play-large', 'play', 'progress', 'current-time', 'duration', 'mute', 'volume', 'captions', 'settings', 'pip', 'airplay', 'fullscreen'
      ],
      settings: ['captions', 'quality', 'speed', 'loop']
    });
  } else if (isAudio.value && audioEl.value) {
    plyrInstance.value = new Plyr(audioEl.value, {
      autoplay: true,
      controls: [
        'play-large', 'play', 'progress', 'current-time', 'duration', 'mute', 'volume', 'settings', 'loop'
      ]
    });
  }

  if (plyrInstance.value) {
    // Restore progress
    if (currentMedia.value) {
      try {
        const time = await api.getProgress(currentMedia.value.id);
        if (time > 0) {
          plyrInstance.value.once('ready', () => {
            plyrInstance.value!.currentTime = time;
          });
        }
      } catch (e) {
        console.warn('Failed to load progress', e);
      }
    }

    // Save progress event
    plyrInstance.value.on('timeupdate', (event: CustomEvent) => {
      const player = event.detail.plyr;
      if (player.currentTime > 0 && !player.paused) {
        saveProgress(player.currentTime);
      }
    });

    // Volume persistence
    plyrInstance.value.volume = store.volume;
    plyrInstance.value.on('volumechange', () => {
      if (plyrInstance.value) {
        store.setPref('volume', plyrInstance.value.volume.toString());
      }
    });
  }
};

watch(currentMedia, () => {
  initPlayer();
}, { immediate: true });

onUnmounted(() => {
  if (plyrInstance.value) {
    plyrInstance.value.destroy();
  }
});
</script>

<template>
  <div class="player-container">
    <!-- Empty State -->
    <div v-if="!currentMedia" class="empty-state">
      <div class="empty-icon">
        <svg xmlns="http://www.w3.org/2000/svg" width="64" height="64" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1" stroke-linecap="round" stroke-linejoin="round"><rect x="2" y="3" width="20" height="14" rx="2" ry="2"></rect><line x1="8" y1="21" x2="16" y2="21"></line><line x1="12" y1="17" x2="12" y2="21"></line></svg>
      </div>
      <h2>Select a file to preview</h2>
    </div>

    <!-- Video Player -->
    <div v-else-if="isVideo" class="player-wrapper">
      <video ref="videoEl" class="plyr-video" crossorigin="anonymous">
        <source :src="streamUrl" :type="'video/' + currentMedia.ext.replace('.', '')" />
        <track 
          v-for="sub in currentMedia.subtitles" 
          :key="sub.id"
          kind="captions" 
          :label="sub.label" 
          :src="api.getSubtitleUrl(currentMedia.id) + '&sid=' + sub.id" 
          :default="sub.default"
        />
        <!-- Also add generic track if subtitles exist but logic differs -->
      </video>
    </div>

    <!-- Audio Player -->
    <div v-else-if="isAudio" class="player-wrapper audio-layout">
      <div class="audio-cover" v-if="currentMedia.coverId">
         <!-- Placeholder for cover art API if it exists, otherwise use generic icon -->
         <!-- Assuming streamUrl with some param or separate endpoint for cover -->
         <!-- For now, just a placeholder icon -->
         <svg xmlns="http://www.w3.org/2000/svg" width="100" height="100" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1" stroke-linecap="round" stroke-linejoin="round"><path d="M9 18V5l12-2v13"></path><circle cx="6" cy="18" r="3"></circle><circle cx="18" cy="16" r="3"></circle></svg>
      </div>
      <div class="audio-main">
        <h3>{{ currentMedia.name }}</h3>
        <audio ref="audioEl" class="plyr-audio" crossorigin="anonymous">
          <source :src="streamUrl" />
        </audio>
      </div>
    </div>

    <!-- Image Viewer -->
    <div v-else-if="isImage" class="image-viewer">
      <img :src="streamUrl" :alt="currentMedia.name" />
    </div>
  </div>
</template>

<style scoped>
.player-container {
  width: 100%;
  height: 100%;
  display: flex;
  flex-direction: column;
  justify-content: center;
  align-items: center;
  min-height: 400px;
  background: #000; /* Dark background for player area */
  border-radius: 16px;
  overflow: hidden;
  position: relative;
}

.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  color: var(--md-sub);
  background: var(--md-surface);
  width: 100%;
  height: 100%;
  justify-content: center;
}

.empty-icon {
  margin-bottom: 20px;
  opacity: 0.5;
}

.player-wrapper {
  width: 100%;
  max-height: 100%;
  display: flex;
  justify-content: center;
}

/* Video specific overrides */
:deep(.plyr) {
  width: 100%;
  height: 100%;
}

:deep(.plyr__video-wrapper) {
  height: 100%;
}

.image-viewer {
  width: 100%;
  height: 100%;
  display: flex;
  justify-content: center;
  align-items: center;
  background: #0d1117;
}

.image-viewer img {
  max-width: 100%;
  max-height: 100%;
  object-fit: contain;
}

/* Audio Layout */
.audio-layout {
  flex-direction: column;
  align-items: center;
  padding: 40px;
  gap: 30px;
  background: var(--md-surface);
  height: 100%;
}

.audio-cover {
  width: 200px;
  height: 200px;
  background: var(--md-bg);
  border-radius: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--md-sub);
  border: 1px solid var(--md-border);
}

.audio-main {
  width: 100%;
  max-width: 600px;
  text-align: center;
}

.audio-main h3 {
  margin-bottom: 20px;
  color: var(--md-text);
}
</style>
