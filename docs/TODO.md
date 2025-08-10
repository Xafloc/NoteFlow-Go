# NoteFlow Development Log & TODOs

## Current Sprint

### In Progress
- [x] Architecture analysis of Python implementation
- [x] Technology stack research for Go version
- [ ] Create Go project structure and initial implementation

### Blocked
- [ ] Nothing currently blocked

### Up Next
- [ ] Implement core note management with goldmark
- [ ] Build Fiber web server with embedded assets
- [ ] Add task/checkbox management system
- [ ] Implement website archiving functionality
- [ ] Add theme system and persistence
- [ ] Cross-platform build and distribution setup

## Completed

### Week of 2025-01-07
- [x] Analyzed existing Python noteflow.py architecture
- [x] Researched optimal Go technology stack
- [x] Created development documentation structure

## Notes

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