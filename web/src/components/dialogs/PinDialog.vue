<script setup lang="ts">
import { ref, onMounted, nextTick } from 'vue';
import { useMainStore } from '@/stores/main';

const store = useMainStore();
const pin = ref('');
const error = ref('');
const inputEl = ref<HTMLInputElement | null>(null);

const submit = async () => {
  if (!pin.value) return;
  
  error.value = '';
  const success = await store.verifyPin(pin.value);
  
  if (success) {
    pin.value = '';
  } else {
    error.value = 'Invalid PIN';
    pin.value = '';
    // shake animation or focus
    inputEl.value?.focus();
  }
};

onMounted(() => {
  nextTick(() => {
    inputEl.value?.focus();
  });
});
</script>

<template>
  <div v-if="store.showPin" class="dialog__backdrop">
    <div class="dialog" style="width: 320px;">
      <div class="dialog__title">Authentication Required</div>
      <div class="dialog__body">
        <p style="margin-top: 0; color: var(--md-sub);">Please enter the server PIN to access media.</p>
        
        <input 
          ref="inputEl"
          v-model="pin" 
          type="password" 
          class="textfield" 
          placeholder="PIN" 
          style="text-align: center; font-size: 24px; letter-spacing: 4px;"
          @keyup.enter="submit"
        >
        
        <div v-if="error" style="color: var(--md-danger); font-size: 13px; margin-top: 8px; text-align: center;">
          {{ error }}
        </div>
      </div>
      <div class="dialog__actions">
        <button class="btn" style="width: 100%;" @click="submit">Unlock</button>
      </div>
    </div>
  </div>
</template>
