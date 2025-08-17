# NoteFlow Development Log & TODOs

## Current Sprint - Week of 2025-08-14

### In Progress
- [ ] Address any remaining bugs or performance issues
- [ ] Consider next enhancement features from backlog

### Blocked
- [ ] Nothing currently blocked

### Up Next
- [ ] WebSocket implementation for real-time updates
- [ ] Full-text search functionality
- [ ] Export to PDF/HTML feature
- [ ] Plugin system architecture

## Completed

### Week of 2025-08-14
- [x] Added comprehensive note collapse/expand functionality
- [x] Implemented hover menu with collapse controls on note headers
- [x] Created individual collapse/expand for each note with click-anywhere-to-expand
- [x] Added collapse all, expand all, and focus (collapse others) operations
- [x] Designed distinct menus for expanded vs collapsed states
- [x] Styled collapsed notes with greyed-out headers and hidden content
- [x] Fixed menu positioning and visibility issues
- [x] Updated TODO.md management rules and documentation standards
- [x] Implemented tagged release system for stable Homebrew distribution
- [x] Added comprehensive version management procedures to CLAUDE.md
- [x] Fixed version constant/tag alignment issues that caused user confusion
- [x] Validated end-to-end Homebrew installation and version reporting

### Week of 2025-08-12
- [x] Fixed binary naming consistency to noteflow-go
- [x] Updated README with latest features and improvements
- [x] Added full path tooltips and click-to-copy for folder names on global tasks page
- [x] Fixed MathJax rendering for sidebar tasks
- [x] Enhanced website archiving with comprehensive resource inlining

### Week of 2025-08-10  
- [x] Implemented website archiving for +http links
- [x] Added automatic cleanup for stale folders in global task registry
- [x] Fixed automatic port detection for multiple instances
- [x] Updated documentation for noteflow-go binary name

### Week of 2025-01-07 (Project Foundation)
- [x] Analyzed existing Python noteflow.py architecture
- [x] Researched optimal Go technology stack  
- [x] Created complete Go project structure
- [x] Implemented core note management with goldmark
- [x] Built Fiber web server with embedded assets
- [x] Added task/checkbox management system
- [x] Implemented website archiving functionality
- [x] Added theme system and persistence
- [x] Set up cross-platform build and distribution
- [x] Created Homebrew tap for distribution

## Notes

### Recent Decisions & Rollbacks
- **Formatting Issues Rollback (Aug 2025)**: Rolled back some TODO formatting changes due to rendering/display issues. Maintained core functionality while reverting problematic formatting decisions to ensure TODO.md remains readable and functional.

### Architecture Decisions
- **Web Framework**: Fiber chosen for performance and simplicity
- **Markdown Parser**: goldmark with extensions for CommonMark compliance
- **Asset Embedding**: Go 1.16+ embed package for zero-dependency distribution
- **Database**: SQLite for cross-project task tracking + file-based notes storage
- **WebSocket**: For real-time updates and multi-client synchronization
- **Build**: Standard Go toolchain + GoReleaser for cross-platform builds

### Technical Debt
- Need to implement proper file locking for concurrent access
- Website archiving needs improved error handling and timeout management
- Theme system should be more extensible

### Performance Optimizations
- Target startup time: <50ms (improvement over Python's 100ms target)
- Target memory usage: 8-10MB baseline (improvement over Python's 15MB)
- Implement efficient markdown caching for large files
- Use goroutines for non-blocking website archiving

### Known Issues & Solutions
- **Cross-project task synchronization**: Use embedded SQLite database with file watchers
- **Static asset serving**: Embed all assets in binary using Go embed package
- **MathJax integration**: Continue using client-side rendering for offline support
- **File encoding issues**: Go's UTF-8 default should resolve Python's encoding problems

### Testing Strategy
- Unit tests for core note/task management
- Integration tests for web server endpoints
- Cross-platform build verification
- Performance benchmarking against Python version
- Manual testing checklist for all UI features

### Development Priorities
1. **Phase 1**: Core functionality (notes, tasks, markdown rendering)
2. **Phase 2**: Web interface and theme system
3. **Phase 3**: Website archiving and file uploads
4. **Phase 4**: Cross-project task management
5. **Phase 5**: Distribution and packaging