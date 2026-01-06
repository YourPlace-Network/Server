window.bootstrap = require("bootstrap/dist/js/bootstrap.bundle");
import "../../scss/components/modalPasswordCreate.scss";

/*
 Usage:
  import { ShowModalPasswordCreate } from "./components/modalPasswordCreate";

  const password = await ShowModalPasswordCreate();
  if (password) {
      // User entered a valid password
  }

  Template inclusion:
  {{template "modalPasswordCreate" .}}
 */

const MIN_PASSWORD_LENGTH = 7;

let DOM = {
    cancelBtn: null as HTMLButtonElement | null,
    confirmInput: null as HTMLInputElement | null,
    errorDiv: null as HTMLDivElement | null,
    passwordInput: null as HTMLInputElement | null,
    strengthFill: null as HTMLDivElement | null,
    strengthLabel: null as HTMLDivElement | null,
    submitBtn: null as HTMLButtonElement | null,
};
let modal: bootstrap.Modal;
let resolvePromise: ((value: string | null) => void) | null = null;

interface PasswordStrength {
    entropy: number;
    label: string;
    level: "weak" | "fair" | "good" | "strong";
    percentage: number;
}

function calculatePasswordStrength(password: string): PasswordStrength {
    if (password.length === 0) {
        return {entropy: 0, label: "Enter a password", level: "weak", percentage: 0};
    }
    if (password.length < MIN_PASSWORD_LENGTH) {
        return {entropy: 0, label: `Too short (min ${MIN_PASSWORD_LENGTH} chars)`, level: "weak", percentage: 5};
    }
    let poolSize = 0;
    if (/[a-z]/.test(password)) poolSize += 26;
    if (/[A-Z]/.test(password)) poolSize += 26;
    if (/[0-9]/.test(password)) poolSize += 10;
    if (/[^a-zA-Z0-9]/.test(password)) poolSize += 32;
    const entropy = password.length * Math.log2(poolSize);
    let label: string;
    let level: "weak" | "fair" | "good" | "strong";
    let percentage: number;
    if (entropy < 40) {
        label = `Weak (${Math.round(entropy)} bits)`;
        level = "weak";
        percentage = Math.min(25, (entropy / 40) * 25);
    } else if (entropy < 60) {
        label = `Fair (${Math.round(entropy)} bits)`;
        level = "fair";
        percentage = 25 + ((entropy - 40) / 20) * 25;
    } else if (entropy < 80) {
        label = `Good (${Math.round(entropy)} bits)`;
        level = "good";
        percentage = 50 + ((entropy - 60) / 20) * 25;
    } else {
        label = `Strong (${Math.round(entropy)} bits)`;
        level = "strong";
        percentage = Math.min(100, 75 + ((entropy - 80) / 40) * 25);
    }
    return {entropy, label, level, percentage};
}
function cleanup(): void {
    DOM.cancelBtn!.removeEventListener("click", handleCancel);
    DOM.confirmInput!.removeEventListener("input", validateInputs);
    DOM.passwordInput!.removeEventListener("input", handlePasswordInput);
    DOM.submitBtn!.removeEventListener("click", handleSubmit);
    DOM.confirmInput!.value = "";
    DOM.errorDiv!.textContent = "";
    DOM.passwordInput!.value = "";
    DOM.strengthFill!.className = "password-strength-fill";
    DOM.strengthFill!.style.width = "0%";
    DOM.strengthLabel!.textContent = "Enter a password";
    DOM.submitBtn!.disabled = true;
}
function handleCancel(): void {
    cleanup();
    modal.hide();
    if (resolvePromise) {
        resolvePromise(null);
        resolvePromise = null;
    }
}
function handlePasswordInput(): void {
    const password = DOM.passwordInput!.value;
    const strength = calculatePasswordStrength(password);
    DOM.strengthFill!.className = "password-strength-fill strength-" + strength.level;
    DOM.strengthFill!.style.width = strength.percentage + "%";
    DOM.strengthLabel!.textContent = strength.label;
    validateInputs();
}
function handleSubmit(): void {
    const password = DOM.passwordInput!.value;
    cleanup();
    modal.hide();
    if (resolvePromise) {
        resolvePromise(password);
        resolvePromise = null;
    }
}
function validateInputs(): void {
    const password = DOM.passwordInput!.value;
    const confirm = DOM.confirmInput!.value;
    if (password.length < MIN_PASSWORD_LENGTH) {
        DOM.errorDiv!.textContent = "";
        DOM.submitBtn!.disabled = true;
        return;
    }
    if (confirm.length > 0 && password !== confirm) {
        DOM.errorDiv!.textContent = "Passwords do not match";
        DOM.submitBtn!.disabled = true;
        return;
    }
    DOM.errorDiv!.textContent = "";
    DOM.submitBtn!.disabled = password.length < MIN_PASSWORD_LENGTH || password !== confirm;
}

export function HideModalPasswordCreate(): void {
    cleanup();
    modal.hide();
}
export function ShowModalPasswordCreate(): Promise<string | null> {
    return new Promise((resolve) => {
        resolvePromise = resolve;
        DOM.cancelBtn!.addEventListener("click", handleCancel);
        DOM.confirmInput!.addEventListener("input", validateInputs);
        DOM.passwordInput!.addEventListener("input", handlePasswordInput);
        DOM.submitBtn!.addEventListener("click", handleSubmit);
        modal.show();
    });
}

(function initialize() {
    if (document.readyState === "loading") {document.addEventListener("DOMContentLoaded", main);} else {main();}
    function main() {
        const element = document.getElementById("modalPasswordCreate");
        if (!element) return;
        DOM = {
            cancelBtn: document.getElementById("passwordCreateCancelBtn") as HTMLButtonElement,
            confirmInput: document.getElementById("passwordCreateConfirmInput") as HTMLInputElement,
            errorDiv: document.getElementById("passwordCreateError") as HTMLDivElement,
            passwordInput: document.getElementById("passwordCreateInput") as HTMLInputElement,
            strengthFill: document.getElementById("passwordStrengthFill") as HTMLDivElement,
            strengthLabel: document.getElementById("passwordStrengthLabel") as HTMLDivElement,
            submitBtn: document.getElementById("passwordCreateSubmitBtn") as HTMLButtonElement,
        };
        modal = new window.bootstrap.Modal(element, {
            backdrop: "static",
            keyboard: false
        });
        element.addEventListener("hide.bs.modal", () => {
            if (element.contains(document.activeElement)) {
                (document.activeElement as HTMLElement).blur();
            }
        });
    }
})();
