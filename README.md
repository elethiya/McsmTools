# MCSM TOOLS

**MCSM TOOLS** is a modern, lightweight, and high-performance Minecraft Server Control Center and Dashboard. Built with a powerful Go backend and a highly responsive custom-themed frontend, it provides an intuitive interface for server administrators to deploy, manage, and monitor multiple Minecraft server instances seamlessly.

## 🚀 Features

- **Real-Time Console**: Integrated WebSocket connection providing live server logs and the ability to execute console commands instantaneously.
- **Advanced Server Management**: Create, swap, start, stop, and kill active server instances directly from the UI.
- **Analytics & Metrics**: Live hardware usage (CPU/RAM) monitoring using Chart.js to keep track of server performance.
- **File Manager**: Built-in web-based file explorer and code editor (powered by CodeMirror) for easily editing properties, configs, and JSON files.
- **1-Click Installers**: Download and install server software directly from official sources (Vanilla, Paper, etc.) as well as popular plugins (e.g., Geyser for Bedrock cross-play).
- **Player Management**: Monitor online players, manage operator status, and control whitelists/bans effortlessly.
- **Futuristic UI/UX**: 
  - Entirely custom dark-mode interface styled with a distinct orange gradient theme.
  - Angular, military-style typography using the **Quantico** font.
  - Fully responsive and mobile-friendly design without relying on heavy frontend frameworks.

## 🛠️ Tech Stack

- **Backend**: Go (Golang)
- **Frontend**: HTML5, Vanilla JavaScript, Custom CSS
- **Libraries/Integrations**: 
  - [Chart.js](https://www.chartjs.org/) (for analytics)
  - [CodeMirror](https://codemirror.net/) (for the file editor)
  - [Google Fonts (Quantico)](https://fonts.google.com/specimen/Quantico)

## 📂 Project Structure

- `main.go`: The core application entry point and HTTP server.
- `/pkg/`: Modular Go packages handling specific systems (`auth`, `config`, `filemgr`, `installers`, `mcserver`, `websocket`, etc.)
- `/web/templates/`: HTML templates served by the backend (e.g., `index.html`, `login.html`).
- `/web/static/`: Static assets, including the custom `style.css`, `app.js`, and `assets/` (logos/favicons).
- `/servers/`: The default directory where Minecraft server instances and their data are stored.

## ⚙️ Setup & Installation

1. Clone the repository to your local machine or server.
2. Ensure you have [Go](https://go.dev/) installed.
3. Download dependencies:
   ```bash
   go mod tidy
   ```
4. Run the application:
   ```bash
   go run main.go
   ```
5. Navigate to `http://localhost:8080` (or your configured port) in your browser.
6. Login using your configured administrator credentials.

---
*Built for absolute control. Designed for high performance.*
