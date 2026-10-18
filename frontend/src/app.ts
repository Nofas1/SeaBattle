import { ApiError, clearToken, setBaseURL, setToken } from "./api/client";
import { initAuth, handleUnauthorized, logout, restoreSession } from "./auth";
import { initBots } from "./bots";
import { hideEndModal, refreshGameState, renderBattle } from "./battle";
import { initPlacement, renderPlacement } from "./placement";
import { resetGame, state, type ScreenName } from "./state";

let toastTimeout = 0;

function showScreen(screen: ScreenName): void {
  state.screen = screen;
  for (const name of ["auth", "bots", "placement", "battle"] as const) {
    document.querySelector<HTMLElement>(`#screen-${name}`)?.classList.toggle("hidden", name !== screen);
  }
  if (screen === "placement") {
    renderPlacement();
  } else if (screen === "battle") {
    renderBattle();
  }
}

function showToast(message: string): void {
  const toast = document.querySelector<HTMLDivElement>("#toast");
  if (toast === null) {
    return;
  }
  toast.textContent = message;
  toast.classList.remove("hidden");
  window.clearTimeout(toastTimeout);
  toastTimeout = window.setTimeout(() => toast.classList.add("hidden"), 5000);
}

function onUnauthorized(): void {
  logout();
  resetGame();
  hideEndModal();
  navigate("auth");
  showToast("Your session expired. Please sign in again.");
}

function onAuthenticated(): void {
  navigate("bots");
  window.dispatchEvent(new Event("bots:reload"));
}

function handleApiError(error: unknown): void {
  if (error instanceof ApiError && error.status === 401) {
    handleUnauthorized({
      onAuthenticated,
      onUnauthorized,
      onError: showToast,
    });
    return;
  }
  showToast(error instanceof Error ? error.message : String(error));
}

function routeFromHash(): void {
  if (state.token === null) {
    showScreen("auth");
    return;
  }
  if (window.location.hash === "#screen-bots") {
    showScreen("bots");
  } else if (window.location.hash === "#screen-placement" && state.game !== null) {
    showScreen("placement");
  } else if (window.location.hash === "#screen-battle" &&
      state.game !== null && state.game.sessionId !== null) {
    showScreen("battle");
    void refreshGameState();
  } else {
    navigate("bots");
  }
}

function navigate(screen: ScreenName): void {
  const hash = `#screen-${screen}`;
  if (window.location.hash !== hash) {
    window.location.hash = hash;
  }
  showScreen(screen);
  if (screen === "battle") {
    void refreshGameState();
  }
}

function boot(): void {
  const apiBaseURL = window.location.port === "8080"
    ? `${window.location.protocol}//${window.location.hostname}:28080`
    : "";
  setBaseURL(apiBaseURL);
  const token = localStorage.getItem("sb_token");
  if (token !== null) {
    setToken(token);
    state.token = token;
  } else {
    clearToken();
    state.token = null;
  }

  initAuth({
    onAuthenticated,
    onUnauthorized,
    onError: showToast,
  });
  initBots(() => navigate("placement"), handleApiError);
  initPlacement(() => navigate("battle"), handleApiError);
  document.querySelector<HTMLButtonElement>("#retry-state-button")?.addEventListener("click", () => {
    void refreshGameState();
  });

  document.querySelector<HTMLButtonElement>("#logout-button")?.addEventListener("click", () => {
    logout();
    resetGame();
    navigate("auth");
  });
  document.querySelector<HTMLButtonElement>("#play-again-button")?.addEventListener("click", () => {
    hideEndModal();
    resetGame();
    navigate("bots");
    window.dispatchEvent(new Event("bots:reload"));
  });

  document.addEventListener("keydown", (event) => {
    if (event.key === "Enter" && !document.querySelector("#modal-end")?.classList.contains("hidden")) {
      document.querySelector<HTMLButtonElement>("#play-again-button")?.click();
    }
  });

  window.addEventListener("auth:unauthorized", onUnauthorized);
  window.addEventListener("app:toast", (event) => {
    if (event instanceof CustomEvent && typeof event.detail === "string") {
      showToast(event.detail);
    }
  });
  window.addEventListener("hashchange", routeFromHash);
  window.addEventListener("screenchange", (event) => {
    if (event instanceof CustomEvent && isScreenName(event.detail)) {
      navigate(event.detail);
    }
  });

  void restoreSession({
    onAuthenticated,
    onUnauthorized: () => {
      state.token = null;
      showScreen("auth");
    },
    onError: showToast,
  }).then(() => {
    if (state.token === null) {
      showScreen("auth");
    }
  }).catch(handleApiError);
}

function isScreenName(value: unknown): value is ScreenName {
  return value === "auth" || value === "bots" || value === "placement" || value === "battle";
}

document.addEventListener("DOMContentLoaded", boot, { once: true });
