<script setup lang="ts">
import { onMounted } from 'vue';
import { useMainStore } from '@/stores/main';
import Topbar from '@/components/layout/Topbar.vue';
import Sidebar from '@/components/layout/Sidebar.vue';
import Player from '@/components/player/Player.vue';
import SettingsDialog from '@/components/dialogs/SettingsDialog.vue';
import PinDialog from '@/components/dialogs/PinDialog.vue';

const store = useMainStore();

onMounted(() => {
  store.init();
});
</script>

<template>
  <Topbar />
  
  <div class="layout">
    <Sidebar />
    
    <main class="stage">
      <div class="panel" style="flex: 1; min-height: 0;">
        <Player />
      </div>
    </main>
  </div>

  <SettingsDialog />
  <PinDialog />
</template>

<style scoped>
</style>
