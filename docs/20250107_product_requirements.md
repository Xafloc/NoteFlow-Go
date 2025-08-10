# NoteFlow Product Requirements Document
*Created: January 7, 2025*

## Executive Summary

NoteFlow is a fast, lightweight, cross-platform note-taking application designed for developers, writers, and power users who want a local-first solution without cloud dependencies. This Go implementation aims to improve upon the existing Python version with better performance, easier distribution, and enhanced cross-project task management.

## Product Vision

**Mission**: Provide the fastest, most efficient local note-taking experience with markdown support, task management, and website archiving capabilities.

**Key Differentiators**:
- Single binary deployment with zero dependencies
- Cross-project task synchronization
- Website archiving with full resource inlining
- Beautiful, functional web interface
- Sub-second startup times

## Target Users

### Primary Users
- **Developers**: Need quick note-taking during coding sessions with markdown support
- **Technical Writers**: Require markdown editing with live preview and MathJax support  
- **Project Managers**: Want task tracking across multiple project folders
- **Researchers**: Need website archiving and reference management

### User Stories
1. "As a developer, I want to quickly jot down notes in any project folder without setup"
2. "As a technical writer, I want to write markdown with math formulas and see live preview"
3. "As a project manager, I want to see all my tasks from different projects in one view"
4. "As a researcher, I want to archive web articles with all resources for offline reading"

## Core Features

### 1. Note Management
- **Markdown Editing**: Full CommonMark support with live preview
- **File Storage**: Single `notes.md` per project folder with custom delimiter format
- **Timestamps**: Automatic timestamping with sortable display
- **Search**: Full-text search across all notes (future enhancement)

### 2. Task Management
- **Checkbox Support**: Standard markdown task syntax `- [ ]` and `- [x]`
- **Cross-Project Views**: Central dashboard showing tasks from all NoteFlow folders
- **Bi-directional Sync**: Complete tasks in either project view or central dashboard
- **Persistence**: Task states maintained across application restarts

### 3. Website Archiving
- **URL Processing**: Convert `+https://example.com` to archived local copies
- **Resource Inlining**: Embed all CSS, JS, images, and fonts as data URIs
- **Offline Viewing**: Self-contained HTML files work without internet
- **Metadata Storage**: Store tags, timestamps, and descriptions

### 4. Media Management
- **Drag & Drop**: Upload images and files via drag-and-drop interface
- **Auto-linking**: Generate appropriate markdown links for uploaded content
- **Asset Organization**: Store in `assets/images/` and `assets/files/` directories

### 5. Theme System
- **Multiple Themes**: dark-orange, dark-blue, light-blue themes
- **Persistence**: Save user theme preferences
- **CSS Variables**: Dynamic theme switching without reload
- **Extensibility**: Support for custom theme definitions

## Technical Requirements

### Performance Requirements
- **Startup Time**: < 50ms from launch to server ready
- **Memory Usage**: < 10MB baseline memory consumption
- **Response Times**: 
  - API endpoints: < 10ms for CRUD operations
  - Markdown rendering: < 50ms for typical notes
  - Page loads: < 100ms for UI updates

### Compatibility Requirements
- **Operating Systems**: Windows 10+, macOS 10.15+, Linux (major distributions)
- **Browsers**: Chrome 80+, Firefox 75+, Safari 13+, Edge 80+
- **File Systems**: Support for all major file systems (NTFS, APFS, ext4, etc.)

### Security Requirements
- **Local-only**: No network communication except for website archiving
- **File System Security**: Restrict operations to working directory
- **Input Validation**: Sanitize all user inputs to prevent XSS
- **Archive Safety**: Validate URLs and prevent SSRF attacks

## User Interface Requirements

### Layout
- **Two-column Design**: Note editing on left, tasks/links on right
- **Responsive**: Adapt to different window sizes
- **Collapsible Sections**: Focus mode for note editing

### Interaction Patterns
- **Keyboard Shortcuts**: Ctrl+Enter to save, Tab support in textarea
- **Drag & Drop**: File uploads with visual feedback
- **Real-time Updates**: Live preview of markdown rendering
- **Loading States**: Progress indicators for long operations

### Accessibility
- **Keyboard Navigation**: Full functionality via keyboard
- **Screen Reader Support**: Proper ARIA labels and semantic HTML
- **High Contrast**: Themes support accessibility standards

## Data Requirements

### Storage Format
```markdown
## 2025-01-07 14:30:00 - Note Title
Note content with **markdown** formatting.
- [ ] Task item
- [x] Completed task

<!-- note -->

## 2025-01-07 14:25:00
Another note without title.
More content here.

<!-- note -->
```

### File Organization
```
project-folder/
├── notes.md              # Main notes file
├── assets/
│   ├── images/          # Uploaded images
│   ├── files/           # Uploaded documents
│   └── sites/           # Archived websites
│       ├── *.html       # Archived pages
│       └── *.tags       # Metadata files
└── .noteflow/           # Hidden config (future)
```

### Cross-Project Database
```sql
-- SQLite schema for global task tracking
CREATE TABLE projects (
    id INTEGER PRIMARY KEY,
    path TEXT UNIQUE,
    name TEXT,
    last_accessed DATETIME
);

CREATE TABLE tasks (
    id INTEGER PRIMARY KEY,
    project_id INTEGER,
    content TEXT,
    completed BOOLEAN,
    created_at DATETIME,
    updated_at DATETIME,
    FOREIGN KEY (project_id) REFERENCES projects(id)
);
```

## Integration Requirements

### Build System
- **Go Modules**: Standard dependency management
- **Asset Embedding**: Use `embed` package for static assets
- **Cross-compilation**: Support all target platforms from single source

### Distribution Channels
- **GitHub Releases**: Primary distribution with checksums
- **Package Managers**: Homebrew (macOS/Linux), Scoop (Windows)
- **Direct Download**: Single executable from project website

## Quality Requirements

### Reliability
- **Data Integrity**: Atomic file operations with proper locking
- **Error Handling**: Graceful degradation on file system errors
- **Backup Strategy**: Automatic backups before major operations

### Maintainability
- **Code Organization**: Clean architecture with separated concerns
- **Testing Coverage**: >80% test coverage for core functionality
- **Documentation**: Comprehensive API and architecture documentation

### Usability
- **Zero Configuration**: Works out-of-box in any directory
- **Intuitive Interface**: Minimal learning curve for new users
- **Error Messages**: Clear, actionable error messages

## Success Metrics

### Performance Metrics
- Startup time: < 50ms (vs Python's ~500ms actual)
- Memory usage: < 10MB (vs Python's ~25MB actual)
- Binary size: < 15MB (vs Python's ~100MB with dependencies)

### User Experience Metrics
- Time to first note: < 5 seconds from launch
- Note save time: < 100ms including markdown render
- Website archive time: < 30 seconds for typical pages

### Adoption Metrics
- GitHub stars growth rate
- Download counts from releases
- Community contributions and issues

## Future Enhancements

### Phase 2 Features
- **Plugin System**: Extensible architecture for custom features
- **Export Options**: PDF, HTML, and other format exports
- **Advanced Search**: Full-text search with filtering and tagging
- **Vim Keybindings**: Optional vim-style editing mode

### Phase 3 Features
- **Real-time Collaboration**: Multiple users editing same notes
- **Cloud Sync**: Optional cloud backup and synchronization
- **Mobile Companion**: Read-only mobile app for note access
- **API Extensions**: RESTful API for third-party integrations

## Constraints

### Technical Constraints
- **No External Dependencies**: Must work as single binary
- **File-based Storage**: No database server requirements
- **Local-first**: No mandatory network connectivity
- **Cross-platform**: Must work identically on all supported platforms

### Business Constraints
- **Open Source**: MIT license for maximum adoption
- **Resource Limitations**: Single developer initially
- **Timeline**: MVP ready within 4-6 weeks