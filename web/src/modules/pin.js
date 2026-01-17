import { el } from './state.js';
import { t } from './i18n.js';
import { apiPost } from './api.js';

export function showPinDialog() {
  const backdrop = el("pinBackdrop");
  const dialog = el("pinDlg");
  const input = el("pinInput");
  const errorEl = el("pinError");

  backdrop.hidden = false;
  dialog.hidden = false;

  // Update i18n
  el("pinDlgTitle").textContent = t("pin_title");
  el("pinDlgNote").textContent = t("pin_note");
  input.placeholder = t("pin_placeholder");
  el("btnSubmitPin").textContent = t("pin_submit");
  errorEl.textContent = "";

  // Focus input
  setTimeout(() => input.focus(), 100);
}

export function hidePinDialog() {
  el("pinBackdrop").hidden = true;
  el("pinDlg").hidden = true;
  el("pinInput").value = "";
  el("pinError").textContent = "";
}

export async function verifyPin(pin) {
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
    const response = await fetch("/api/config", { cache: "no-store" });

    if (response.status === 401) {
      // PIN is required
      return true;
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

  submitBtn.addEventListener("click", handleSubmit);
  input.addEventListener("keypress", (e) => {
    if (e.key === "Enter") {
      handleSubmit();
    }
  });
}
