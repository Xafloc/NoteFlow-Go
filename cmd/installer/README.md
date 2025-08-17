# NoteFlow-Go Installer

A cross-platform installer for NoteFlow-Go that downloads the latest release and sets up the application with optional PATH configuration.

## Features

- 🔍 **Automatic Detection**: Detects your platform and downloads the correct binary
- 📁 **Interactive Setup**: Choose your installation directory  
- 🛤️ **PATH Management**: Optionally adds NoteFlow-Go to your PATH
- 🔒 **No Admin Required**: Works without administrator privileges
- 🌍 **Cross-Platform**: Works on Windows, macOS, and Linux

## Usage

### Windows
1. Download `noteflow-installer-windows-amd64.exe` from [releases](https://github.com/Xafloc/NoteFlow-Go/releases)
2. Double-click to run or open PowerShell and run:
   ```powershell
   .\noteflow-installer-windows-amd64.exe
   ```

### macOS  
1. Download `noteflow-installer-darwin-amd64` from [releases](https://github.com/Xafloc/NoteFlow-Go/releases)
2. Make executable and run:
   ```bash
   chmod +x noteflow-installer-darwin-amd64
   ./noteflow-installer-darwin-amd64
   ```

### Linux
1. Download `noteflow-installer-linux-amd64` from [releases](https://github.com/Xafloc/NoteFlow-Go/releases)
2. Make executable and run:
   ```bash
   chmod +x noteflow-installer-linux-amd64  
   ./noteflow-installer-linux-amd64
   ```

## Installation Process

The installer will:

1. **Fetch Latest Release**: Downloads the most recent NoteFlow-Go release
2. **Choose Directory**: Presents installation directory options:
   - Windows: `%USERPROFILE%\bin`, `%USERPROFILE%\tools`, `%USERPROFILE%\Apps`, `%USERPROFILE%\Desktop`
   - macOS/Linux: `~/bin`, `~/.local/bin`, `~/tools`, `/usr/local/bin`
   - Custom path option
3. **Download Binary**: Downloads and installs the appropriate binary
4. **PATH Setup**: Optionally adds installation directory to your PATH
5. **Verification**: Confirms successful installation

## PATH Management

### Windows
- Modifies User PATH environment variable (no admin required)
- Changes take effect after restarting terminal/PowerShell

### macOS/Linux  
- Adds export line to shell configuration files (`.bashrc`, `.bash_profile`, `.zshrc`, `.profile`)
- Changes take effect after restarting terminal or running `source ~/.bashrc`

## Example Installation

```
NoteFlow-Go Installer v1.0.0
========================================

Installing NoteFlow-Go v1.1.9

Choose installation directory:
1. C:\Users\john\bin (recommended)
2. C:\Users\john\tools
3. C:\Users\john\Apps
4. C:\Users\john\Desktop
5. Custom path

Enter choice (1-5): 1

Downloading noteflow-go-windows-amd64.exe...
✓ Installed to: C:\Users\john\bin\noteflow.exe

Add to PATH so you can run 'noteflow' from anywhere? (y/n): y
✓ Added to PATH
  Please restart your terminal/PowerShell for PATH changes to take effect

Installation complete!
Run 'noteflow' from any directory to start the application
Or run directly: C:\Users\john\bin\noteflow.exe
```

## Building from Source

```bash
cd cmd/installer
go build -o noteflow-installer .
```