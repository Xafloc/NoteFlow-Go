# AI Coding Assistant Guidelines - NoteFlow

This document contains guidelines for AI coding assistants. It is structured with generic best practices that apply to all projects, followed by project-specific sections for the NoteFlow application.

## Generic Guidelines (All Projects)

### Project Management

#### Documentation Hierarchy

1. **CLAUDE.md is the source of truth** for all project specifications
2. README.md derives from CLAUDE.md with human-friendly summaries
3. In README.md, include: "For detailed specifications, see CLAUDE.md"
4. Update CLAUDE.md first, then sync README.md for major changes only
5. Keep CLAUDE.md in project root
6. **Use docs/ folder for topical documentation**:
   - Store technical docs, product requirements, and design specs
   - Use lowercase snake_case with date prefix (e.g., 20250107_architecture.md)
   - **Required docs**: `docs/TODO.md` and `docs/20YYMMDD_product_requirements.md` must be created for all projects
   - Maintain an index of these documents in both CLAUDE.md and README.md
   - **AI INSTRUCTION**: Proactively create TODO.md and product_requirements.md if missing, then notify the user

#### TODO.md as Development Log

1. **IMPORTANT**: Maintain `docs/TODO.md` as central development log and task tracker
2. Include:
   - Current/upcoming tasks (with checkboxes)
   - Development decisions and rationale
   - Recurring issues and their solutions
   - Technical debt items
   - Code audit reminders
3. Update when: starting tasks, making progress, completing tasks, encountering issues
4. Check at start of each session

#### Session Start Checklist

1. Check `docs/TODO.md` for current state and development log
2. Run `git status` to see uncommitted changes
3. Run code tracking commands (see Maintenance section)
4. Review recent commits: `git log --oneline -5`
5. Check for outdated dependencies (if applicable)

#### Git Usage (Solo Developer)

1. Create git repo for each project with .gitignore
2. Work directly on main branch (no need for feature branches with multiple AI agents sharing same filesystem)
3. Conventional commit format: "type: Brief description" (fix, feat, docs, refactor, test, chore)
4. Before committing: run lint/build checks and verify functionality
5. Keep main branch stable and deployable
6. Commit regularly to track changes and enable rollback if needed

### Development Workflow

#### Planning and Implementation

1. Discuss approach and evaluate pros/cons before coding
2. Make small, testable incremental changes
3. Address code duplication proactively
4. When fixing issues, check for similar problems elsewhere
5. Document recurring issues in TODO.md

#### Code Quality Standards

1. Handle errors properly and validate inputs
2. Follow code conventions and established patterns
3. Never expose secrets/keys
4. Write self-documenting code with type safety
5. Remove debug output before production
6. **Documentation**: Be concise but accurate - avoid verbose explanations
   - Code comments only when necessary for complex logic
   - Documentation should be clear, direct, and to the point
   - Avoid redundant comments that merely describe what code already shows
   - Focus on "why" not "what" when documenting

#### Code Maintenance and Refactoring

1. **AI Tracking Instructions**:
   - Before each session: run `git diff --stat` to see recent changes
   - Track cumulative additions: `git log --numstat --pretty=format:'' | awk '{ add += $1 } END { print add }'`
   - When total additions exceed 500 lines since last audit, prompt user:
     "Added ~500+ lines since last code audit. Should we review for refactoring opportunities?"
   - Add to TODO.md: "Code audit pending (X lines added since DATE)"
2. Automatic refactoring triggers:
   - Duplicate code blocks (3+ occurrences)
   - Functions > 50 lines
   - Files > 300 lines (except main.go which may be larger)
   - Multiple similar error handlers
3. During audits, check for:
   - Extractable shared utilities
   - Complex functions to split
   - Dead code to remove
   - Performance bottlenecks
4. Track findings in TODO.md under "Notes > Technical Debt"
5. Always refactor before major feature additions

### Debugging and Logging

1. **Use structured logging** instead of fmt.Println
2. Remove debug statements before committing
3. Clean up debug output before production
4. Use appropriate log levels (trace, debug, info, warn, error)
5. For Go: use zerolog or zap for structured logging

## Go-Specific Guidelines

### Language Standards

1. **Go Version**: Go 1.21+ (for latest embed and generics features)
2. **Code Style**: Follow `gofmt` and `go vet` standards
3. **Linting**: Use `golangci-lint` with strict configuration
4. **Dependencies**: Minimal external dependencies, prefer standard library
5. **Error Handling**: Explicit error handling, no panic in production code
6. **Concurrency**: Use goroutines and channels for async operations
7. **Memory Management**: Avoid memory leaks, use context for cancellation

### Code Organization

1. **Package Layout**: Follow standard Go project layout
2. **Internal Packages**: Use internal/ for non-exported packages
3. **Command Pattern**: Main package minimal, logic in internal packages
4. **Interface Design**: Small, focused interfaces
5. **Dependency Injection**: Use constructor functions
6. **Testing**: Co-located test files with `_test.go` suffix

### Performance Considerations

1. **Memory Allocation**: Minimize allocations, reuse buffers
2. **String Operations**: Use strings.Builder for concatenation
3. **I/O Operations**: Buffer reads/writes, use sync.Pool for buffers
4. **Goroutine Management**: Limit concurrent goroutines
5. **Profiling**: Use pprof for performance analysis
6. **Benchmarking**: Write benchmarks for critical paths

## Project-Specific Sections

### Project Overview

**Project Name**: NoteFlow

**Purpose**: A fast, lightweight, cross-platform note-taking application with markdown support, designed to run from any folder and create a web-based interface for managing notes in a single markdown file.

**Key Features**:
1. Markdown note-taking with live preview and MathJax support
2. Task/checkbox management with persistent state
3. Task/checkbox management across all NoteFlow documents
  - While each project folder hosts a single markdown file for notes/tasks/etc., there should be a web interface to view all tasks across all NoteFlow managed documents
  - Completion in individual projects or the central management interface should complete the task in both locations (e.g., project folder MD and central web interface)
  - Need tech recommendations on how to complete this
4. Website archiving with full resource inlining
5. Drag-and-drop file/image uploads
6. Multiple color themes with persistence
7. Single-file storage (notes.md) in working directory
8. Zero-dependency deployment (single binary if possible)
9. Cross-platform support (Windows, macOS, Linux)
10. Beautiful rich web environment that is pleasing to the eye and very functional

**Target Users**: Developers, writers, and power users who want a fast, local note-taking solution without cloud dependencies

**High-Level Architecture**:
1. **Backend**: Fiber web framework with embedded assets (Go embed package)
2. **Frontend**: Vanilla JS with embedded HTML/CSS, MathJax for math rendering
3. **Storage**: Single markdown file per project + SQLite for cross-project task tracking
4. **Rendering**: goldmark with extensions for CommonMark compliance
5. **Distribution**: Single binary with all assets embedded, GoReleaser for builds
6. **Persistence**: SQLite database in user config directory for global task state

### Directory Structure

```
noteflow-go/
├── cmd/
│   └── noteflow/
│       └── main.go          # Application entry point
├── internal/
│   ├── app/                 # Application core
│   │   ├── server.go        # Fiber server setup
│   │   └── config.go        # Configuration management
│   ├── handlers/            # HTTP handlers
│   │   ├── notes.go         # Note CRUD operations
│   │   ├── tasks.go         # Task management
│   │   ├── archive.go       # Website archiving
│   │   └── themes.go        # Theme management
│   ├── models/              # Data models
│   │   ├── note.go          # Note structure and methods
│   │   ├── task.go          # Task structure and methods
│   │   └── database.go      # Database models
│   ├── services/            # Business logic
│   │   ├── notemanager.go   # Note management service
│   │   ├── taskmanager.go   # Task synchronization service
│   │   ├── archiver.go      # Website archiving service
│   │   └── renderer.go      # Markdown rendering service
│   └── storage/             # Storage layer
│       ├── file.go          # File operations
│       └── sqlite.go        # SQLite operations
├── web/                     # Embedded web assets
│   ├── static/
│   │   ├── css/
│   │   ├── js/
│   │   └── fonts/
│   └── templates/
│       └── index.html
├── docs/                    # Documentation
│   ├── TODO.md             # Development log
│   └── 20250107_product_requirements.md
├── go.mod
├── go.sum
├── .gitignore
├── .goreleaser.yml          # Release configuration
└── README.md
```

**Organization Patterns**:
1. **Layered Architecture**: Handlers → Services → Storage
2. **Dependency Injection**: Constructor functions with interface parameters
3. **Error Propagation**: Explicit error handling at each layer
4. **Configuration**: Environment-based config with sensible defaults
5. **Asset Embedding**: All web assets embedded in binary using embed package

### Project Documentation Index

#### Required Documents (Created):
- `docs/TODO.md` - Development log and task tracker ✓
- `docs/20250107_product_requirements.md` - Product requirements and specifications ✓

#### Recommended Documents:
- `docs/ARCHITECTURE.md` - System architecture and design decisions
- `docs/THEME_SYSTEM.md` - Theme configuration and customization
- `docs/ARCHIVING_SPEC.md` - Website archiving implementation details
- `docs/DISTRIBUTION.md` - Build and distribution process

### Code Organization Patterns

**Package Structure**:
1. `cmd/noteflow/` - Application entry point and CLI handling
2. `internal/app/` - Application configuration and server setup
3. `internal/handlers/` - HTTP request handlers (Fiber routes)
4. `internal/models/` - Data structures and domain objects
5. `internal/services/` - Business logic and coordination
6. `internal/storage/` - File and database operations
7. `web/` - Static assets for embedding

**Naming Conventions**:
1. **Packages**: lowercase, single word (notemanager, not note_manager)
2. **Types**: PascalCase (NoteManager, TaskService)
3. **Functions**: PascalCase for exported, camelCase for unexported
4. **Constants**: SCREAMING_SNAKE_CASE for package-level constants
5. **Variables**: camelCase, descriptive names
6. **Files**: lowercase with underscores (note_manager.go)
7. **Interfaces**: -er suffix when possible (Renderer, Archiver)

**Error Handling Patterns**:
- **Wrap Errors**: Use fmt.Errorf("operation failed: %w", err)
- **Custom Errors**: Define package-specific error types
- **HTTP Errors**: Return appropriate HTTP status codes with error messages
- **Logging**: Log errors at origin, don't log and return
- **Recovery**: Graceful degradation, never panic in production
- **Validation**: Return validation errors early in request handling

**Concurrency Patterns**:
1. **Website Archiving**: Use goroutines with context cancellation
2. **File Operations**: Mutex locks for concurrent file access
3. **Database Operations**: Connection pooling with proper timeouts
4. **Worker Pools**: Limited goroutines for resource-intensive operations
5. **Channel Communication**: Use channels for goroutine coordination
6. **Context Propagation**: Pass context through all async operations

### Domain-Specific Guidelines

**Note Storage Format**:
**EXAMPLE: UPDATE THIS SECTION ONCE TECH HAS BEEN ESTABLISHED**
```markdown
## 2024-03-15 09:30:45 - Title
Note content here with **markdown** support.
- [ ] Task item
- [x] Completed task

<!-- note -->

## 2024-03-14 15:22:10
Another note without title.
```

**Business Logic Rules**:
1. **Note Ordering**: Newest notes first (prepend to file)
2. **Task Persistence**: Maintain task state across edits
3. **Delimiter**: Use `<!-- note -->` as note separator
4. **Timestamps**: ISO format YYYY-MM-DD HH:MM:SS
5. **File Locking**: Implement proper file locking for concurrent access

**Archiving Strategy**:
1. **Resource Inlining**: Convert all external resources to data URIs
2. **Self-Contained**: Archived pages work offline
3. **Naming Convention**: `YYYY_MM_DD_HHMMSS_title-domain.html`
4. **Storage Location**: `assets/sites/` in working directory
5. **Metadata**: Store .tags file with URL, title, timestamp

**Theme System**:
1. **Available Themes**: dark-orange, dark-blue, light-blue
2. **Storage**: JSON config in user config directory
3. **CSS Variables**: Dynamic theme switching without reload
4. **Custom Themes**: Extensible theme definition structure

### Performance Requirements

**Startup Performance**:
- Target: < 50ms from launch to server ready (improved from Python)
- Load notes lazily if file > 10MB
- Cache parsed markdown for large files

**Memory Usage**:
- Target: < 10MB baseline memory usage (improved from Python)
- Efficient string handling for large notes
- Reuse buffers for markdown rendering

**Response Times**:
- API responses: < 10ms for CRUD operations (improved from Python)
- Markdown rendering: < 100ms for typical notes
- Archive operations: Async with progress indication

**Scalability Limits**:
- Support up to 10,000 notes in single file
- Handle individual notes up to 1MB
- Archive sites up to 50MB (with resource inlining)

### Security Considerations

**Input Validation**:
1. Sanitize markdown input to prevent XSS
2. Validate file uploads (size, type restrictions)
3. Escape HTML in user content
4. Validate archive URLs (prevent SSRF)

**File System Security**:
1. Restrict file operations to working directory
2. Prevent directory traversal attacks
3. Validate file paths and names
4. Set appropriate file permissions (0644)

**Network Security**:
1. Local-only binding by default (localhost)
2. Optional network binding with warning
3. CORS headers for API endpoints
4. Rate limiting for archive operations

### Testing Strategy

**Unit Testing**:
- Use standard Go testing package
- Table-driven tests for multiple scenarios
- Mock interfaces using testify/mock or manual mocks
- Test files co-located with source files
- Focus on business logic and edge cases

**Test Coverage Goals**:
- **Core Logic**: >90% coverage for services and models
- **HTTP Handlers**: >80% coverage with integration tests
- **Overall Project**: >80% total coverage
- **Critical Paths**: 100% coverage for data persistence and archiving

**Integration Tests**:
1. **HTTP API**: Test full request/response cycles
2. **File Operations**: Test file creation, modification, deletion
3. **Database Operations**: Test SQLite operations with temporary DB
4. **Cross-Project Tasks**: Test task synchronization across projects
5. **Website Archiving**: Test with local test server

**Manual Testing Checklist**:
- [ ] Application startup and shutdown
- [ ] Note creation, editing, deletion in web interface
- [ ] Markdown rendering with all supported syntax
- [ ] Task checkbox functionality and persistence
- [ ] Create, edit, delete notes
- [ ] Task checkbox functionality
- [ ] Drag-and-drop file upload
- [ ] Website archiving (+http links)
- [ ] Theme switching and persistence
- [ ] Cross-platform binary execution

### Deployment and Environment

**Build Process**:
- `go build` for development builds
- `go generate` for asset embedding
- GoReleaser for cross-platform distribution builds
- GitHub Actions for automated CI/CD
- Docker support for containerized deployment

**Distribution Channels**:
1. **GitHub Releases**: Primary distribution with checksums
2. **Homebrew**: macOS/Linux formula
3. **Scoop**: Windows package manager
4. **Direct Download**: Single binary from website

**Version Management**:
- Semantic versioning (v1.0.0, v1.1.0, etc.)
- Git tags for releases
- Version embedded in binary at build time
- Changelog generation with conventional commits

**Installation Methods**:
- Direct binary download from GitHub releases
- `go install github.com/user/noteflow-go/cmd/noteflow@latest`
- Homebrew formula for macOS/Linux
- Scoop package for Windows
- Docker image for containerized usage

## Development Setup

### First Time Setup

**Prerequisites**:
1. Go 1.21 or later
2. Git for version control
3. Make (optional, for build automation)
4. golangci-lint for code quality
5. GoReleaser for release builds

**Setup Steps**:
1. `git clone <repository-url>`
2. `cd noteflow-go`
3. `go mod download`
4. `go generate ./...` (embed assets)
5. `go build -o noteflow cmd/noteflow/main.go`
6. `./noteflow` to run in current directory

### Common Commands

**Development**:
- `go run cmd/noteflow/main.go` - Run application
- `go generate ./...` - Regenerate embedded assets
- `air` - Live reload during development (optional)
- `go test ./...` - Run all tests
- `go test -race ./...` - Run tests with race detection

**Building**:
- `go build -o noteflow cmd/noteflow/main.go` - Development build
- `go build -ldflags="-s -w" -o noteflow cmd/noteflow/main.go` - Optimized build
- `goreleaser build --snapshot --rm-dist` - Cross-platform builds
- `goreleaser release --rm-dist` - Release build with artifacts

**Code Quality**:
- `go fmt ./...` - Format code
- `go vet ./...` - Static analysis
- `golangci-lint run` - Comprehensive linting
- `go test -cover ./...` - Test coverage
- `go mod tidy` - Clean up dependencies

**Release Process** (REQUIRED for every push to main):
1. **Before pushing to GitHub:**
   - Run full test suite and quality checks
   - Update version numbers if this is a release-worthy change
2. **After pushing to GitHub:**
   - **ALWAYS create a tagged release** (required for Homebrew)
   - Increment version appropriately: patch (1.0.1), minor (1.1.0), major (2.0.0)
   - Create and push git tag: `git tag v1.x.x -m "Release v1.x.x: Brief description"`
   - Push tag: `git push origin v1.x.x`
3. **Update Homebrew formula** (in homebrew-tap/homebrew-noteflow-go/Formula/noteflow.rb):
   - Change `url` to point to new tag: `v1.x.x.tar.gz`
   - Calculate new `sha256`: `curl -sL https://github.com/Xafloc/NoteFlow-Go/archive/v1.x.x.tar.gz | sha256sum`
   - Update `version "1.x.x"`
   - Update feature descriptions if needed
4. **Commit and push Homebrew formula changes**
5. **Optional:** GoReleaser for additional distribution (GitHub releases, etc.)

**CRITICAL:** Never leave main branch commits without corresponding tags - this breaks Homebrew installations!

### Feature Parity Checklist

**Core Features**:
- [x] Markdown note creation and editing
- [x] Task/checkbox management (per initiated project folder, and centralized management)
- [x] Note collapsing and focusing
- [x] Timestamp-based organization
- [x] Single file storage (notes.md)

**Advanced Features**:
- [x] Website archiving with +http prefix
- [x] Resource inlining for offline viewing
- [x] Drag-and-drop file uploads
- [x] Multiple theme support
- [x] Theme persistence
- [x] MathJax rendering

**UI/UX Features**:
- [x] Responsive design
- [x] Keyboard shortcuts (Ctrl+Enter to save)
- [x] Tab key support in textarea
- [x] Auto-expanding textarea
- [x] Loading indicators

**New Improvements**:
- [ ] WebSocket for real-time updates
- [ ] Full-text search
- [ ] Export to PDF/HTML
- [ ] Plugin system
- [ ] Vim keybindings (optional)

## Appendix

### TODO.md Management Rules

**Update Frequency:**
1. **Weekly Reviews**: Review and update TODO.md every week
2. **Session Start**: Check TODO.md at beginning of each development session
3. **Task Completion**: Update immediately when tasks are completed
4. **Rollback Documentation**: When rolling back features/decisions, document reasoning in Notes section

**Content Requirements:**
1. **Current Sprint**: Active tasks with realistic timelines
2. **Completed Sections**: Move finished items with completion dates
3. **Notes Sections**: Document decisions, rollbacks, and technical debt
4. **Blocked Items**: Include with reason and required resolution
5. **Version Alignment**: Keep TODO.md aligned with actual codebase state

**Maintenance Rules:**
- Never leave TODO.md out of sync with project reality
- Document rollback decisions immediately when they occur
- Archive completed sprints monthly to prevent file bloat
- Use clear, actionable language for all todo items

### TODO.md Template

```markdown
# NoteFlow Development Log & TODOs

## Current Sprint - Week of YYYY-MM-DD

### In Progress
- [ ] Current active tasks

### Blocked
- [ ] Blocked items with reasons

### Up Next
- [ ] Prioritized upcoming tasks

## Completed

### Week of YYYY-MM-DD
- [x] Recently completed items with dates

## Notes

### Recent Decisions & Rollbacks
- Document any feature rollbacks or major decisions

### Architecture Decisions
- Key technical decisions and rationale

### Technical Debt
- Known issues requiring future attention

### Performance Optimizations
- Completed and planned optimizations

### Known Issues & Solutions
- Recurring problems and their fixes
```

---

**Remember**: This document should evolve with the project. Update project-specific sections as you make architectural decisions and establish patterns. The Go implementation should maintain feature parity with the Python version while improving performance and distribution.