import { listBots } from "./api/client";
import { resetGame, setGameBot } from "./state";

export function initBots(
  onSelect: (botId: string) => void,
  onError: (error: unknown) => void,
): void {
  const list = document.querySelector<HTMLDivElement>("#bot-list");
  if (list === null) {
    return;
  }

  const load = (): void => {
    void listBots().then(({ bots }) => {
      list.replaceChildren();
      for (const botId of bots) {
        const card = document.createElement("article");
        card.className = "bot-card";
        const title = document.createElement("h2");
        title.textContent = botId;
        const description = document.createElement("p");
        description.className = "subtle";
        description.textContent = "Choose this opponent to start a game.";
        const select = document.createElement("button");
        select.type = "button";
        select.className = "button button-primary";
        select.textContent = "Choose opponent";
        select.addEventListener("click", () => {
          resetGame();
          setGameBot(botId);
          onSelect(botId);
        });
        card.append(title, description, select);
        list.append(card);
      }
      if (bots.length === 0) {
        const empty = document.createElement("p");
        empty.className = "subtle";
        empty.textContent = "No bots are configured on the server.";
        list.append(empty);
      }
    }).catch(onError);
  };
  window.addEventListener("bots:reload", load);
}
