import type { Pair } from "./api/types";

export type CellState = "water" | "ship" | "hit" | "miss" | "sunk" | "around-sunk" | "disabled";

export interface BoardOptions {
  label: string;
  getCellState: (x: number, y: number) => CellState;
  isInteractive?: (x: number, y: number) => boolean;
  onCellClick?: (x: number, y: number) => void;
  onCellHover?: (x: number, y: number, cell: HTMLButtonElement) => void;
}

export function renderBoard(container: HTMLElement, options: BoardOptions): void {
  const fragment = document.createDocumentFragment();
  container.replaceChildren();
  container.setAttribute("aria-label", options.label);

  for (let y = 0; y < 10; y += 1) {
    for (let x = 0; x < 10; x += 1) {
      const interactive = options.isInteractive?.(x, y) === true;
      const element = interactive || options.onCellClick !== undefined
        ? document.createElement("button")
        : document.createElement("div");
      const cellState = options.getCellState(x, y);
      element.className = `cell cell-${cellState}`;
      element.dataset.x = String(x);
      element.dataset.y = String(y);
      element.dataset.state = cellState;
      element.setAttribute("aria-label", `${options.label}: ${x + 1}, ${y + 1}, ${cellState}`);
      element.setAttribute("role", "gridcell");

      if (element instanceof HTMLButtonElement) {
        element.type = "button";
        element.disabled = !interactive;
        if (interactive) {
          element.addEventListener("click", () => options.onCellClick?.(x, y));
          element.addEventListener("pointerenter", () => options.onCellHover?.(x, y, element));
        }
      }
      fragment.append(element);
    }
  }
  container.replaceChildren(fragment);
  container.onpointerleave = () => clearPlacementPreview(container);
}

export function showPlacementPreview(
  board: HTMLElement,
  field: readonly (readonly number[])[],
  startX: number,
  startY: number,
  size: number,
  direction: Pair,
): void {
  clearPlacementPreview(board);
  let valid = true;
  const previewCells: HTMLElement[] = [];

  for (let offset = 0; offset < size; offset += 1) {
    const x = startX + direction.x * offset;
    const y = startY + direction.y * offset;
    const cell = getCell(board, x, y);
    if (cell === null) {
      valid = false;
      continue;
    }
    previewCells.push(cell);
    if (!canPlaceAt(field, x, y)) {
      valid = false;
    }
  }
  for (const cell of previewCells) {
    cell.classList.add(valid ? "placement-preview-valid" : "placement-preview-invalid");
  }
}

export function clearPlacementPreview(board: HTMLElement): void {
  board.querySelectorAll(".placement-preview-valid, .placement-preview-invalid").forEach((cell) => {
    cell.classList.remove("placement-preview-valid", "placement-preview-invalid");
  });
}

function getCell(board: HTMLElement, x: number, y: number): HTMLElement | null {
  if (x < 0 || x >= 10 || y < 0 || y >= 10) {
    return null;
  }
  return board.querySelector<HTMLElement>(`.cell[data-x="${x}"][data-y="${y}"]`);
}

function canPlaceAt(field: readonly (readonly number[])[], x: number, y: number): boolean {
  if (x < 0 || x >= 10 || y < 0 || y >= 10) {
    return false;
  }
  for (let neighborX = x - 1; neighborX <= x + 1; neighborX += 1) {
    for (let neighborY = y - 1; neighborY <= y + 1; neighborY += 1) {
      if (neighborX >= 0 && neighborX < 10 && neighborY >= 0 && neighborY < 10 &&
          field[neighborX]?.[neighborY] === 1) {
        return false;
      }
    }
  }
  return true;
}
