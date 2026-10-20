import { clearToken, login, register, setToken, verify } from "./api/client";
import { state } from "./state";

export interface AuthCallbacks {
  onAuthenticated: () => void;
  onUnauthorized: () => void;
  onError: (message: string) => void;
}

export function initAuth(callbacks: AuthCallbacks): void {
  const loginTab = document.querySelector<HTMLButtonElement>("#auth-tab-login");
  const registerTab = document.querySelector<HTMLButtonElement>("#auth-tab-register");
  const form = document.querySelector<HTMLFormElement>("#auth-form");
  const submit = document.querySelector<HTMLButtonElement>("#auth-submit");
  const nameInput = document.querySelector<HTMLInputElement>("#auth-name");
  const passwordInput = document.querySelector<HTMLInputElement>("#auth-password");
  const formError = document.querySelector<HTMLParagraphElement>("#auth-form-error");

  loginTab?.addEventListener("click", () => setAuthMode("login"));
  registerTab?.addEventListener("click", () => setAuthMode("register"));
  form?.addEventListener("submit", async (event) => {
    event.preventDefault();
    clearErrors();
    const name = nameInput?.value.trim() ?? "";
    const password = passwordInput?.value ?? "";
    let valid = true;

    if (name.length === 0) {
      setFieldError("name", "Enter your username.");
      valid = false;
    }
    if (password.length === 0) {
      setFieldError("password", "Enter your password.");
      valid = false;
    } else if (password.length < 8) {
      setFieldError("password", "Password must be at least 8 characters.");
      valid = false;
    }
    if (!valid) {
      return;
    }

    if (submit !== null) {
      submit.disabled = true;
    }
    try {
      if (state.authMode === "register") {
        await register(name, password);
        setAuthMode("login");
        if (formError !== null) {
          formError.textContent = "Account created. Sign in to continue.";
          formError.classList.add("success-message");
        }
        return;
      }

      const result = await login(name, password);
      const token = result.token;
      if (typeof token !== "string" || token.length === 0) {
        throw new Error("The server did not return an authentication token.");
      }
      saveToken(token);
      callbacks.onAuthenticated();
    } catch (error) {
      if (isUnauthorized(error)) {
        clearToken();
        localStorage.removeItem("sb_token");
        state.token = null;
        callbacks.onUnauthorized();
        return;
      }
      callbacks.onError(errorMessage(error));
    } finally {
      if (submit !== null) {
        submit.disabled = false;
      }
    }
  });
}

export async function restoreSession(
  callbacks: AuthCallbacks,
): Promise<void> {
  const token = localStorage.getItem("sb_token");
  if (token === null || token.length === 0) {
    clearToken();
    state.token = null;
    return;
  }

  setToken(token);
  state.token = token;
  try {
    await verify();
    callbacks.onAuthenticated();
  } catch (error) {
    clearToken();
    localStorage.removeItem("sb_token");
    state.token = null;
    callbacks.onUnauthorized();
    if (isUnauthorized(error)) {
      return;
    }
    callbacks.onError(errorMessage(error));
  }
}

export function logout(): void {
  localStorage.removeItem("sb_token");
  clearToken();
  state.token = null;
}

export function handleUnauthorized(callbacks: AuthCallbacks): void {
  logout();
  callbacks.onUnauthorized();
}

function saveToken(token: string): void {
  localStorage.setItem("sb_token", token);
  setToken(token);
  state.token = token;
}

function setAuthMode(mode: "login" | "register"): void {
  state.authMode = mode;
  const loginTab = document.querySelector<HTMLButtonElement>("#auth-tab-login");
  const registerTab = document.querySelector<HTMLButtonElement>("#auth-tab-register");
  const submit = document.querySelector<HTMLButtonElement>("#auth-submit");
  const password = document.querySelector<HTMLInputElement>("#auth-password");
  const formError = document.querySelector<HTMLParagraphElement>("#auth-form-error");
  loginTab?.classList.toggle("active", mode === "login");
  loginTab?.setAttribute("aria-selected", String(mode === "login"));
  registerTab?.classList.toggle("active", mode === "register");
  registerTab?.setAttribute("aria-selected", String(mode === "register"));
  if (submit !== null) {
    submit.textContent = mode === "login" ? "Sign in" : "Create account";
  }
  password?.setAttribute("autocomplete", mode === "login" ? "current-password" : "new-password");
  formError?.classList.remove("success-message");
  if (formError !== null) {
    formError.textContent = "";
  }
  clearErrors();
}

function setFieldError(field: "name" | "password", message: string): void {
  const input = document.querySelector<HTMLInputElement>(`#auth-${field}`);
  const error = document.querySelector<HTMLElement>(`[data-error-for="${field}"]`);
  input?.setAttribute("aria-invalid", "true");
  if (error !== null) {
    error.textContent = message;
  }
}

function clearErrors(): void {
  document.querySelectorAll<HTMLInputElement>("#auth-form input").forEach((input) => {
    input.removeAttribute("aria-invalid");
  });
  document.querySelectorAll<HTMLElement>(".field-error").forEach((error) => {
    error.textContent = "";
  });
  const formError = document.querySelector<HTMLParagraphElement>("#auth-form-error");
  if (formError !== null && !formError.classList.contains("success-message")) {
    formError.textContent = "";
  }
}

function isUnauthorized(error: unknown): boolean {
  return typeof error === "object" && error !== null &&
    "status" in error && error.status === 401;
}

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}
