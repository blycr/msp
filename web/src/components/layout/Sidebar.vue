<script setup lang="ts">
import { ref, computed } from 'vue';
import { useMainStore } from '@/stores/main';
import MediaList from '@/components/media/MediaList.vue';
import type { MediaFile } from '@/types';

const store = useMainStore();

const activeTab = ref<string>('video');
const searchQuery = ref('');
const sortOrder = ref<'asc' | 'desc'>('asc');

const tabs = computed(() => {
  const t = [
    { id: 'video', label: 'Video' },
    { id: 'audio', label: 'Audio' },
    { id: 'image', label: 'Image' },
  ];
  if (store.config?.ui?.showOthers) {
    t.push({ id: 'other', label: 'Other' });
  }
  return t;
});

const currentItems = computed(() => {
  switch (activeTab.value) {
    case 'video': return store.videos;
    case 'audio': return store.audios;
    case 'image': return store.images;
    case 'other': return store.others;
    default: return store.videos;
  }
});

const displayItems = computed(() => {
  let items = [...currentItems.value];
  
  if (searchQuery.value) {
    const q = searchQuery.value.toLowerCase();
    items = items.filter(item => 
      item.name.toLowerCase().includes(q) || 
      item.shareLabel.toLowerCase().includes(q)
    );
  }
  
  items.sort((a, b) => {
    const res = a.name.localeCompare(b.name, undefined, { numeric: true, sensitivity: 'base' });
    return sortOrder.value === 'asc' ? res : -res;
  });
  
  return items;
});

const selectItem = (item: MediaFile) => {
  store.playMedia(item);
};

const toggleSort = () => {
  sortOrder.value = sortOrder.value === 'asc' ? 'desc' : 'asc';
};
</script>

<template>
  <aside class="panel sidebar">
    <div class="tabs">
      <div 
        v-for="tab in tabs" 
        :key="tab.id"
        class="tab"
        :class="{ 'tab--active': activeTab === tab.id }"
        @click="activeTab = tab.id"
      >
        {{ tab.label }}
      </div>
    </div>
    
    <div class="search">
      <input 
        v-model="searchQuery" 
        type="text" 
        class="textfield" 
        placeholder="Search..."
      >
      <button class="btn--ghost" @click="toggleSort" title="Sort" style="padding: 10px;">
        <svg v-if="sortOrder === 'asc'" xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m3 16 4 4 4-4"/><path d="M7 20V4"/><path d="M11 4h10"/><path d="M11 8h7"/><path d="M11 12h4"/></svg>
        <svg v-else xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m3 8 4-4 4 4"/><path d="M7 4v16"/><path d="M11 12h4"/><path d="M11 8h7"/><path d="M11 4h10"/></svg>
      </button>
    </div>
    
    <MediaList 
      :items="displayItems" 
      :active-id="store.playingMedia?.id"
      @select="selectItem"
    />
  </aside>
</template>
