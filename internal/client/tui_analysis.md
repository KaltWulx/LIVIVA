# TUI Architecture Analysis: OpenCode vs LIVIVA

This document analyzes the technical principles of OpenCode's TUI and proposes abstractions to improve LIVIVA's user experience and maintainability.

## OpenCode TUI Principles

OpenCode uses a modern, declarative approach to Terminal User Interfaces, diverging from the traditional state-machine-heavy patterns.

### 1. Declarative Component Model
OpenCode uses **SolidJS** with a terminal renderer. This allows developers to use:
- **Reactive Primitives:** `createSignal`, `createMemo` for local state.
- **JSX for Layout:** Tag-like structures (`<box>`, `<text>`) that represent the layout conceptually rather than procedurally.
- **Hooks for Side Effects:** `onMount` and `onCleanup` for lifecycle management.

### 2. Global Dialog Management (`useDialog`)
The `useDialog` hook is a central pillar of OpenCode's TUI.
- **Decoupled Overlays:** Components can trigger dialogs without the parent needing to know about the dialog's state or implementation.
- **Stack-based Navigation:** Dialogs can replace each other or stack, enabling complex wizard-like flows (e.g., `DialogProvider` -> `AutoMethod` -> `DialogModel`).
- **Standardized Closing:** Consistent handling of `Esc` or clicking outside to close.

### 3. Service Layer Integration (`useSync` & `useSDK`)
The TUI is strictly a view layer. 
- **Real-time Sync:** `useSync` ensures the TUI reflects the background "world state" (providers, sessions, agents) without manual polling.
- **SDK Encapsulation:** All logic for API calls or system interaction is hidden behind the SDK.

### 4. Rich Input & Autocomplete (`prompt/`)
The `Prompt` component is a complex system, not just a text box.
- **Extmarks (External Marks):** It tracks "parts" of the prompt (Files, Agents, MCP Resources) using `extmarks`. This allows the text to contain virtual elements like `@Agent` or `file.txt` that carry metadata beyond just the string value.
- **Autocomplete:** A dedicated component that overlays the input, context-aware of the cursor position and trigger characters (`@`, `/`).

### 5. Routing System (`routes/` & `route.tsx`)
OpenCode uses a router to switch between main views (`Home` vs `Session`).
- **Route State:** A global store holds the current route object (e.g., `{ type: "session", sessionID: "..." }`).
- **Switch/Match:** The main `App` component swaps the entire view hierarchy based on the route state.

---

## LIVIVA TUI Status

LIVIVA uses **Bubbletea (TEA)**, which is robust but can become "spaghetti" as complexity grows.

### Current Challenges
- **Monolithic `Update` Function:** `internal/client/tui.go` handles recording, playback, metrics, sending messages, and palette toggling in one place.
- **Tight Coupling:** The main `model` struct holds state for every feature.
- **Manual State Management:** Features like the `CommandPalette` are manually toggled.

---

## Proposed Abstractions for LIVIVA

To "take the best of OpenCode," LIVIVA should adopt a more modular architecture.

### 1. The `Dialog` Interface
Standardize modal components in `pkg/tui`.
```go
type Dialog interface {
    tea.Model
    Title() string
    Active() bool
    Toggle()
}
```

### 2. Centralized `DialogManager`
Instead of hardcoding the palette, use a manager that can host *any* `Dialog` stack.

### 3. Application Router
Implement a top-level Router equivalent to OpenCode's `Switch/Match`.
- **View Models:** Distinct `tea.Model` implementations for `HomeView`, `ChatView` (Session), `SettingsView`.
- **Navigation:** A `NavigateMsg` command to switch views.

### 4. Global Context / Environment
Pass a shared `Context` struct to sub-models containing:
- API Client
- Configuration
- Theme Definition

### 5. Rich Text Input (Long Term)
Refactor the input to support "Chips" or "Parts" similar to OpenCode's `extmarks`.
- Start by abstracting `textarea` into a `PromptComponent` that handles its own autocomplete logic.

## Next Steps
1. Create `pkg/tui/router.go` and `pkg/tui/dialog.go`.
2. Refactor `CommandPalette` to implement `Dialog`.
3. Refactor `tui.go` to use the Router pattern (initially just `ChatView`).
