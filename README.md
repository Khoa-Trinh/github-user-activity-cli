# 📡 GitHub User Activity CLI

A simple **Go command-line tool** to fetch and display a GitHub user’s recent public activity directly in your terminal.

Built without external dependencies — just the Go standard library.

📍 **Project Idea:** [roadmap.sh – GitHub User Activity](https://roadmap.sh/projects/github-user-activity)

---
## ✨ Features

- 🔍 Fetches recent public events from the **GitHub API**.
- 🎯 Optional filtering by event type (e.g., `PushEvent`, `IssuesEvent`).
- 📏 Limit the number of events displayed.
- ⚠️ Graceful error handling (invalid usernames, rate limiting, API errors).
- 🛠 No external libraries — fast and lightweight.

---

## 📦 Installation

### 1. Clone the repository
```bash
git clone https://github.com/Khoa-Trinh/github-user-activity-cli.git
cd github-user-activity-cli
```

### 2. Build the binary
```bash
go build -o github-activity.exe main.go
```

---

## 🚀 Usage

### Basic Command
```bash
./github-activity.exe <username>
```

Example:
```bash
./github-activity.exe Khoa-Trinh
```

### Limit the number of events
```bash
./github-activity.exe --n=5 <username>
```

### Filter by event type
```bash
./github-activity.exe --event=PushEvent <username>
```

### Show help
```bash
./github-activity.exe --help
```

---

## 📝 Example Output

```plaintext
- Pushed 3 commit(s) to torvalds/linux
- Opened an issue #42 “Kernel panic on boot” in torvalds/linux
- Starred golang/go
- Forked alice/project → torvalds/project-fork
```

---

## 🧪 Running Tests
This project comes with unit tests for all major functions, including a mocked GitHub API.

Run all tests:
```bash
go test
```

---

## ⚠️ Rate Limits
GitHub API limits unauthenticated requests to 60 per hour.

To increase this limit, export a personal access token:
```bash
export GITHUB_TOKEN=your_token_here    # Linux/macOS
setx GITHUB_TOKEN your_token_here      # Windows
```

And uncomment the Authorization header line in `fetchEvents()` line 129.:
```go
req.Header.Set("Authorization", "Bearer "+os.Getenv("GITHUB_TOKEN"))
```

---

## 📂 Project Structure
```plaintext
.
├── main.go           # CLI application source
├── main_test.go      # Unit tests (including fetchEvents with mock server)
├── go.mod
└── README.md
```

---

## 🛠 Supported Event Types

- **PushEvent**
- **IssuesEvent**
- **PullRequestEvent**
- **WatchEvent** (stars)
- **ForkEvent**
- **CreateEvent** / **DeleteEvent**
- **ReleaseEvent**
- **PullRequestReviewCommentEvent**
- **IssueCommentEvent**

> Other event types are skipped by default.

---

## 📜 License
This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details

---

**Made with ❤️ in Go**
```yaml

---

If you want, I can also add a **GIF demo** section showing how the CLI works in the terminal — that tends to make GitHub READMEs pop visually.  
Do you want me to make that next?
```