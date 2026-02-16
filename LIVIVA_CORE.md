# LIVIVA: Architectural Roadmap & Strategic Vision
> **Vision:** "Your infrastructure, your control. The intelligence comes from where it is best."
> **Goal:** A local, JARVIS-like second brain that orchestrates the user's digital and physical reality.
## 1. The Hippocampus: Long-Term Memory & Context
**Objective:** LIVIVA must "know" you, not just process your current session. It needs to recall preferences, project history, and device configurations across reboots.
### Architecture
- **Vector Database (Semantic Memory):**
  - **Tech:** `pgvector` (PostgreSQL) or `ChromaDB` (Local).
  - **Usage:** "What did we decide about the IoT protocol last month?" -> Retrieves relevant past conversations and decisions.
- **Relational Database (Structured Facts):**
  - **Tech:** SQLite (embedded).
  - **Usage:** 
    - `UserPreferences` table (e.g., "Language: Spanish", "Editor: VS Code").
    - `DeviceRegistry` table (Known LAN devices, IPs, capabilities).
    - `ProjectMetadata` table (Active workspaces, states).
- **Episodic Memory Loop:**
  - A background process that summarizes finished sessions and stores "Key Insights" into the Vector DB.
- **The Immune System (ADK Callbacks):**
  - **Interceptor Layer:** Logic that runs *before* and *after* every LLM call.
  - **Functions:**
    - `BeforeModel`: Redact PII (Privacy), Inject real-time system stats (Context), Block dangerous instructions (Safety).
    - `BeforeTool`: Confirm destructive commands with user (Human-in-the-loop).
    - `AfterModel`: Hallucination check and format sanitation.
---
## 2. The Nervous System: Universal Connectivity (MCP)
**Objective:** Standardize how LIVIVA interacts with the "outside" world (LAN, APIs, Apps) so it can scale infinitely without rewriting core code.
### Architecture: Model Context Protocol (MCP)
- **Why MCP?** It's the emerging standard (from Anthropic/others) for exposing data and tools to LLMs.
- **Implementation:**
  - **LIVIVA as MCP Host:** The LIVIVA server will act as a host that can connect to any MCP Server.
  - **MCP Servers (The "Drivers"):**
    - `filesystem-mcp`: For safe file access.
    - `postgres-mcp`: To query your databases.
    - `github-mcp`: To interact with your repos.
    - **Custom Drivers:** We will build specific MCP servers for your unique workflows.
- **Benefit:** If you want LIVIVA to use a new tool (e.g., a new calendar app), you just plug in its MCP server. No core recompile needed.
---
## 3. The Senses: Environment Analysis & Perception
**Objective:** LIVIVA must verify reality, not hallucinate it. It needs to "see" the network and system state.
### Modules
- **Network Proprioception (LAN Scanner):**
  - **Tool:** `nmap` wrapper / generic Go net scanner.
  - **Function:** Auto-discovery of devices on `192.168.1.x`.
  - **Active Monitoring:** Alerts when a new unknown device appears.
- **System Introspection (Client Analysis):**
  - **Agent:** A specialized `SysAdminAgent`.
  - **Capabilities:**
    - List running processes/services.
    - Check installed packages/versions.
    - Monitor resource usage (CPU/RAM/Disk).
- **Visual Cortex (Already Started):**
  - Processing screenshots/images from the user (Vision capability).
  - *Future:* Processing RTSP streams from security cameras.
---
## 4. The Hands: Actuation, IoT & Robotics
**Objective:** Move from "Chatting" to "Doing". Controlling the physical world.
### Integrations
- **Home Assistant Bridge:**
  - Instead of reinventing the wheel, LIVIVA connects to Home Assistant API.
  - **Command:** "LIVIVA, turn off the lab lights." -> LLM -> Tool Call -> HA API -> Light.
- **Direct Protocol Control:**
  - **MQTT:** For lightweight IoT messaging.
  - **HTTP/Webhooks:** for generic web-controlled devices.
  - **Serial/Bluetooth:** For direct wearable/robot control.
- **Wearables:**
  - Expose an API endpoint for a Watch/Phone app to send voice/data to LIVIVA.
  - *Concept:* "LIVIVA Watch Link" -> Receives health data or voice commands from your wrist.
## 5. The Forge: Construction & Self-Improvement
**Objective:** LIVIVA helps you build. If it lacks a tool, it helps you create it.
### Meta-Tooling
- **Tool Scaffolder:** An agent capability to generate the boilerplate code for a new LIVIVA Tool.
  - *User:* "I need a tool to control my 3D printer (Octoprint)."
  - *LIVIVA:* "I don't have that. Shall I create a new MCP server template for Octoprint API?"
- **Project Architect:**
  - Manages the lifecycle of your creative projects (not just code, but structure, docs, assets).
---