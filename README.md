# NoteFlow-Go

A fast, lightweight, cross-platform note-taking application with markdown support, designed to run from any folder and create a web-based interface for managing notes in a single markdown file.

## 🚀 Features

- **Markdown Note-Taking**: Live preview with MathJax support for mathematical notation
- **Task Management**: Persistent checkbox/task system with cross-folder synchronization
- **Global Task View**: Manage tasks across all NoteFlow projects from a central interface
- **Website Archiving**: Comprehensive resource inlining with `+http` prefix
- **Drag & Drop**: File and image uploads with automatic asset management
- **Multiple Themes**: Beautiful color schemes with persistence
- **Single File Storage**: All notes stored in `notes.md` in your working directory
- **Zero Dependencies**: Single binary deployment, no external dependencies
- **Cross-Platform**: Works on Windows, macOS, and Linux
- **Fast Performance**: <100ms startup time, <15MB memory usage

## 🆚 Improvements Over Python Version

- **10x Faster Startup**: Go binary vs Python interpreter
- **Lower Memory Usage**: ~15MB vs ~50MB+ for Python version  
- **Cross-Folder Tasks**: SQLite-based task synchronization across projects
- **Single Binary**: No Python runtime or pip dependencies required
- **Better Concurrency**: Native Go routines for background sync
- **Embedded Assets**: All web assets bundled into binary

## 📦 Installation

### Homebrew
```bash
brew install xafloc/noteflow-go/noteflow
```

**Note**: Installs as `noteflow-go` to avoid conflicts with the Python version.

### Easy Installer (Recommended for Windows)
**One-click installation with automatic PATH setup:**

1. Download the installer for your platform from [GitHub Releases](https://github.com/Xafloc/NoteFlow-Go/releases):
   - Windows: `noteflow-installer-windows-amd64.exe`
   - macOS: `noteflow-installer-darwin-amd64` 
   - Linux: `noteflow-installer-linux-amd64`

2. Run the installer:
   ```bash
   # Windows (double-click or run in PowerShell)
   .\noteflow-installer-windows-amd64.exe
   
   # macOS/Linux
   chmod +x noteflow-installer-darwin-amd64
   ./noteflow-installer-darwin-amd64
   ```

3. Follow the interactive prompts to choose installation directory
4. Optionally add to PATH for global access
5. Run `noteflow` from any directory!

**Perfect for users without admin access** - installs to user directory only.

### Direct Download
1. Download the latest release from [GitHub Releases](https://github.com/Xafloc/NoteFlow-Go/releases)
2. Extract and place `noteflow-go` in your PATH
3. Run `noteflow-go` from any directory

### Build from Source
```bash
git clone https://github.com/Xafloc/NoteFlow-Go.git
cd NoteFlow-Go
go build -o noteflow-go .
```

## 🎯 Quick Start

1. **Navigate to any project folder**
   ```bash
   cd ~/my-project
   ```

2. **Start NoteFlow-Go**
   ```bash
   noteflow-go
   ```

3. **Open your browser**
   - Server starts automatically (usually `http://localhost:8000`)
   - Creates `notes.md` in current directory
   - Registers folder for global task management

4. **Create notes and tasks**
   - Write markdown with `- [ ]` for tasks
   - Use `+http://example.com` to archive websites
   - Drag & drop files for uploads

## 🌐 Global Task Management

NoteFlow-Go introduces **cross-folder task synchronization**:

- **Local View**: See tasks for current project folder
- **Global View**: Access `/global-tasks` to see all tasks across all registered folders
- **Two-Way Sync**: Complete tasks from either view
- **Automatic Registration**: Each NoteFlow instance auto-registers its folder
- **Background Sync**: Tasks stay synchronized across all projects
- **Path Navigation**: Hover over folder names to see full paths, click to copy to clipboard

## 🎨 Features in Detail

### Markdown & MathJax
```markdown
# My Research Notes

Calculate eigenvalues for matrix:
$$\lambda_{1,2} = \frac{(a+d) \pm \sqrt{(a+d)^2 - 4(ad-bc)}}{2}$$

## Tasks
- [ ] Complete problem set
- [x] Review lecture notes
```

### Website Archiving
```markdown
+https://example.com/article
```
Creates self-contained HTML with **comprehensive resource inlining**:
- CSS stylesheets and @import rules
- JavaScript files and dependencies  
- Images, fonts, and binary assets (base64 encoded)
- Fully offline-capable archived pages

### File Uploads
Drag any file into the interface - automatically creates `assets/` folder and links.

## 🛠️ Configuration

NoteFlow stores user preferences in `~/.config/noteflow/noteflow.json`:

```json
{
  "theme": "light-blue",
  "port": 8000
}
```

## 🗃️ Directory Structure

```
your-project/
├── notes.md          # All your notes (auto-created)
├── assets/           # Uploaded files (auto-created)
│   ├── images/       # Drag & drop images
│   └── sites/        # Archived websites
└── noteflow-go        # The binary (optional)
```

## 🔧 Development

Built with modern Go technologies:
- **[Fiber](https://gofiber.io/)** - Express.js-inspired web framework
- **[Goldmark](https://github.com/yuin/goldmark)** - CommonMark-compliant markdown parser  
- **[SQLite](https://sqlite.org/)** - Cross-folder task synchronization
- **Embedded Assets** - Single binary with all web resources

### Project Structure
```
noteflow-go/
├── cmd/              # Application entry points
├── internal/         # Private application code
│   ├── models/       # Data structures
│   ├── services/     # Business logic
│   └── handlers/     # HTTP handlers
├── web/              # Frontend assets
│   ├── templates/    # HTML templates
│   └── static/       # CSS, JS, fonts
└── docs/             # Documentation
```

## 📋 Roadmap

- [ ] Full-text search with highlighting (in progress)
- [ ] Plugin system for extensions
- [ ] Export to PDF/HTML
- [ ] Vim keybindings support
- [ ] WebSocket real-time updates
- [ ] Mobile-responsive improvements

## 📄 License

MIT License - see [LICENSE](LICENSE) file for details.

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests if applicable
5. Submit a pull request

## 📬 Support

- **Issues**: [GitHub Issues](https://github.com/Xafloc/NoteFlow-Go/issues)
- **Discussions**: [GitHub Discussions](https://github.com/Xafloc/NoteFlow-Go/discussions)

---

**NoteFlow-Go** - Fast, powerful note-taking for developers and power users. 