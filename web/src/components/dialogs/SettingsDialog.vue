<script setup lang="ts">
import { ref, computed } from 'vue';
import { useMainStore } from '@/stores/main';

const store = useMainStore();

const newSharePath = ref('');
const activeTab = ref<'shares' | 'blacklist' | 'security' | 'server'>('shares');

const shares = computed(() => store.config?.shares || []);
const blacklist = computed(() => store.config?.blacklist || { extensions: [], filenames: [], folders: [], sizeRule: '' });
const security = computed(() => store.config?.security || { ipWhitelist: [], ipBlacklist: [], pinEnabled: false, pin: '0000' });
const server = computed(() => store.config || {});

// Shares
const addShare = async () => {
  if (!newSharePath.value) return;
  try {
    await store.updateShares('add', newSharePath.value);
    newSharePath.value = '';
  } catch (e: any) {
    alert(e.message || 'Failed to add share');
  }
};

const removeShare = async (path: string) => {
  if (!confirm('Remove this share?')) return;
  try {
    await store.updateShares('remove', path);
  } catch (e: any) {
    alert(e.message || 'Failed to remove share');
  }
};

// Generic Array Updates (Blacklist/Security)
const updateList = async (key: string, list: string[]) => {
  if (!store.config) return;
  const newConfig = JSON.parse(JSON.stringify(store.config)); // Deep clone
  
  // Navigate path
  const parts = key.split('.');
  let target = newConfig;
  for (let i = 0; i < parts.length - 1; i++) {
    const keyPart = parts[i] as string;
    target = (target as any)[keyPart];
  }
  const lastKey = parts[parts.length - 1] as string;
  (target as any)[lastKey] = list;
  
  await store.updateConfig(newConfig);
};

const addToList = async (key: string, value: string) => {
  if (!value) return;
  // Get current list
  const parts = key.split('.');
  let list = store.config as any;
  for (const part of parts) list = list[part];
  
  const newList = [...(list as string[]), value];
  await updateList(key, newList);
};

const removeFromList = async (key: string, index: number) => {
  const parts = key.split('.');
  let list = store.config as any;
  for (const part of parts) list = list[part];
  
  const newList = [...(list as string[])];
  newList.splice(index, 1);
  await updateList(key, newList);
};

const newItemValue = ref('');

// Tab Navigation
const tabs = [
  { id: 'shares', label: 'Shares' },
  { id: 'blacklist', label: 'Blacklist' },
  { id: 'security', label: 'Security' },
  { id: 'server', label: 'Server' },
];
</script>

<template>
  <div v-if="store.showSettings" class="dialog__backdrop" @click.self="store.closeSettings()">
    <div class="dialog">
      <div class="dialog__title">Settings</div>
      
      <div class="tabs" style="border-bottom: 1px solid var(--md-border); padding: 0 16px;">
        <div 
          v-for="tab in tabs" 
          :key="tab.id"
          class="tab"
          :class="{ 'tab--active': activeTab === tab.id }"
          style="border: none; border-bottom: 2px solid transparent; border-radius: 0;"
          @click="activeTab = tab.id as any"
        >
          {{ tab.label }}
        </div>
      </div>

      <div class="dialog__body">
        
        <!-- Shares Tab -->
        <div v-if="activeTab === 'shares'">
          <div class="dialog__subtitle">Shared Folders</div>
          <div class="row" style="margin-bottom: 10px;">
            <input 
              v-model="newSharePath" 
              type="text" 
              class="textfield" 
              placeholder="Enter folder path (e.g. D:\Movies)"
              @keyup.enter="addShare"
            >
            <button class="btn" @click="addShare">Add</button>
          </div>
          
          <div class="sharelist">
            <div v-for="share in shares" :key="share.path" class="share">
              <div class="share__main">
                <div>{{ share.label }}</div>
                <div class="share__path">{{ share.path }}</div>
              </div>
              <button class="btn--ghost" @click="removeShare(share.path)" style="color: var(--md-danger); border-color: var(--md-danger)">
                <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M3 6h18"></path><path d="M19 6v14c0 1-1 2-2 2H7c-1 0-2-1-2-2V6"></path><path d="M8 6V4c0-1 1-2 2-2h4c1 0 2 1 2 2v2"></path></svg>
              </button>
            </div>
            <div v-if="shares.length === 0" class="empty">No shares configured</div>
          </div>
        </div>

        <!-- Blacklist Tab -->
        <div v-if="activeTab === 'blacklist'">
           <div class="dialog__subtitle">Extensions to Ignore</div>
           <div class="row" style="margin-bottom: 10px;">
             <input v-model="newItemValue" class="textfield" placeholder=".exe, .dll" @keyup.enter="addToList('blacklist.extensions', newItemValue); newItemValue=''">
             <button class="btn" @click="addToList('blacklist.extensions', newItemValue); newItemValue=''">Add</button>
           </div>
           <div class="tag-list">
             <span v-for="(ext, idx) in blacklist.extensions" :key="idx" class="badge">
               {{ ext }} 
               <span @click="removeFromList('blacklist.extensions', idx)" style="cursor: pointer; margin-left: 4px;">&times;</span>
             </span>
           </div>
           
           <div class="dialog__subtitle" style="margin-top: 20px;">Folders to Ignore</div>
           <div class="row" style="margin-bottom: 10px;">
             <input v-model="newItemValue" class="textfield" placeholder="Folder name" @keyup.enter="addToList('blacklist.folders', newItemValue); newItemValue=''">
             <button class="btn" @click="addToList('blacklist.folders', newItemValue); newItemValue=''">Add</button>
           </div>
           <div class="tag-list">
             <span v-for="(folder, idx) in blacklist.folders" :key="idx" class="badge">
               {{ folder }} 
               <span @click="removeFromList('blacklist.folders', idx)" style="cursor: pointer; margin-left: 4px;">&times;</span>
             </span>
           </div>
        </div>

        <!-- Security Tab -->
        <div v-if="activeTab === 'security'">
           <div class="row" style="justify-content: space-between;">
             <span>PIN Authentication</span>
             <label class="toggle">
               <input 
                 type="checkbox" 
                 :checked="security.pinEnabled" 
                 @change="store.updateConfig({ security: { ...security, pinEnabled: !security.pinEnabled } })"
               >
             </label>
           </div>
           <div v-if="security.pinEnabled" style="margin-top: 10px;">
             <label style="font-size: 13px; color: var(--md-sub);">PIN Code</label>
             <input 
               :value="security.pin" 
               @change="(e) => store.updateConfig({ security: { ...security, pin: (e.target as HTMLInputElement).value } })"
               type="text" 
               class="textfield" 
             >
           </div>
           
           <div class="dialog__subtitle" style="margin-top: 20px;">IP Whitelist</div>
           <div class="hint">Only allow these IPs (leave empty for all)</div>
           <div class="row" style="margin-bottom: 10px;">
             <input v-model="newItemValue" class="textfield" placeholder="192.168.1.5" @keyup.enter="addToList('security.ipWhitelist', newItemValue); newItemValue=''">
             <button class="btn" @click="addToList('security.ipWhitelist', newItemValue); newItemValue=''">Add</button>
           </div>
           <div class="tag-list">
             <span v-for="(ip, idx) in security.ipWhitelist" :key="idx" class="badge">
               {{ ip }} 
               <span @click="removeFromList('security.ipWhitelist', idx)" style="cursor: pointer; margin-left: 4px;">&times;</span>
             </span>
           </div>
        </div>
        
        <!-- Server Tab -->
        <div v-if="activeTab === 'server'">
           <div class="row" style="justify-content: space-between;">
             <span>Port</span>
             <input 
               :value="(server as any).port" 
               @change="(e) => store.updateConfig({ port: parseInt((e.target as HTMLInputElement).value) })"
               type="number" 
               class="textfield" 
               style="width: 100px;"
             >
           </div>
           <div class="hint">Requires restart to take effect</div>
        </div>

      </div>

      <div class="dialog__actions">
        <button class="btn" @click="store.closeSettings()">Close</button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.tag-list {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
.badge {
  display: inline-flex;
  align-items: center;
  background: var(--md-hover);
  color: var(--md-text);
  border-radius: 4px;
  padding: 4px 8px;
  font-size: 13px;
}
</style>
