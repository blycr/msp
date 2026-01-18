<script setup lang="ts">
import type { MediaFile } from '@/types';

defineProps<{
  items: MediaFile[];
  activeId?: string;
}>();

const emit = defineEmits<{
  (e: 'select', item: MediaFile): void
}>();

// Format file size
const formatSize = (bytes: number) => {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
};
</script>

<template>
  <div class="list__body">
    <div 
      v-for="item in items" 
      :key="item.id" 
      class="item"
      :class="{ 'item--active': activeId === item.id }"
      @click="emit('select', item)"
    >
      <div class="item__main">
        <div class="item__name" :title="item.name">{{ item.name }}</div>
        <div class="item__sub">
          {{ formatSize(item.size) }} • {{ item.shareLabel }}
        </div>
      </div>
    </div>
    
    <div v-if="items.length === 0" class="empty">
      No items found
    </div>
  </div>
</template>

<style scoped>
.empty {
  padding: 20px;
  text-align: center;
  color: var(--md-sub);
  font-size: 14px;
}
</style>
