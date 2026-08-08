import { el } from './state.js';
import { t } from './i18n.js';
import { apiPost } from './api.js';

export function showPinDialog() {
  const backdrop = el("pinBackdrop");
  const dialog = el("pinDlg");
  const input = el("pinInput");
  const errorEl = el("pinError");

  backdrop.classList.add("dialog--open");
  dialog.classList.add("dialog--open");

  // Update i18n
  el("pinDlgTitle").textContent = t("pin_title");
  el("pinDlgNote").textContent = t("pin_note");
  input.placeholder = t("pin_placeholder");
  el("btnSubmitPin").textContent = t("pin_submit");
  errorEl.textContent = "";

  // Focus input
  setTimeout(() => input.focus(), 100);
}

function hidePinDialog() {
  el("pinBackdrop").classList.remove("dialog--open");
  el("pinDlg").classList.remove("dialog--open");
  el("pinInput").value = "";
  el("pinError").textContent = "";
}

async function verifyPin(pin) {
  try {
    const data = await apiPost("/api/pin", { pin });
    return data.valid === true;
  } catch (e) {
    console.error("PIN verification failed:", e);
    return false;
  }
}

export async function checkPinRequired() {
  try {
    // Try to access config endpoint to see if PIN is required
    const response = await fetch("/api/config", { cache: "no-store", credentials: "include" });

    if (response.status === 401) {
      // PIN is required and not authenticated
      return true;
    }

    if (response.ok) {
      const data = await response.json();
      // Check if PIN is enabled in config
      if (data.config && data.config.security && data.config.security.pinEnabled) {
        // PIN is enabled, check if we're already authenticated
        // by trying to access a protected endpoint
        const testResponse = await fetch("/api/media", { cache: "no-store", credentials: "include" });
        if (testResponse.status === 401) {
          // Not authenticated, PIN required
          return true;
        }
      }
    }

    // PIN not required or already authenticated
    return false;
  } catch (e) {
    // Network error, assume PIN not required
    return false;
  }
}

export function bindPinDialog() {
  const submitBtn = el("btnSubmitPin");
  const input = el("pinInput");
  const errorEl = el("pinError");
  const form = el("pinForm");

  const handleSubmit = async () => {
    const pin = input.value.trim();
    if (!pin) {
      errorEl.textContent = t("pin_error");
      return;
    }

    submitBtn.disabled = true;
    submitBtn.textContent = t("pin_checking");
    errorEl.textContent = "";

    const valid = await verifyPin(pin);

    if (valid) {
      hidePinDialog();
      // Reload the page to start fresh with authenticated session
      window.location.reload();
    } else {
      errorEl.textContent = t("pin_error");
      submitBtn.disabled = false;
      submitBtn.textContent = t("pin_submit");
      input.value = "";
      input.focus();
    }
  };

  form?.addEventListener("submit", (event) => {
    event.preventDefault();
    handleSubmit();
  });
}
