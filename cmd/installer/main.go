package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

const (
	repoOwner = "Xafloc"
	repoName  = "NoteFlow-Go"
	version   = "1.0.0"
)

type Release struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

func main() {
	fmt.Printf("NoteFlow-Go Installer v%s\n", version)
	fmt.Println("========================================")
	fmt.Println()

	// Get the latest release info
	release, err := getLatestRelease()
	if err != nil {
		fmt.Printf("Error fetching release info: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Installing NoteFlow-Go %s\n\n", release.TagName)

	// Find the appropriate binary for this platform
	binaryName := getBinaryName()
	downloadURL := ""
	for _, asset := range release.Assets {
		if asset.Name == binaryName {
			downloadURL = asset.BrowserDownloadURL
			break
		}
	}

	if downloadURL == "" {
		fmt.Printf("No binary found for %s-%s\n", runtime.GOOS, runtime.GOARCH)
		os.Exit(1)
	}

	// Get installation directory
	installDir := getInstallDirectory()

	// Create directory if it doesn't exist
	if err := os.MkdirAll(installDir, 0755); err != nil {
		fmt.Printf("Error creating directory %s: %v\n", installDir, err)
		os.Exit(1)
	}

	// Download and install
	executableName := getExecutableName()
	installPath := filepath.Join(installDir, executableName)

	fmt.Printf("Downloading %s...\n", binaryName)
	if err := downloadFile(downloadURL, installPath); err != nil {
		fmt.Printf("Error downloading file: %v\n", err)
		os.Exit(1)
	}

	// Make executable on Unix systems
	if runtime.GOOS != "windows" {
		if err := os.Chmod(installPath, 0755); err != nil {
			fmt.Printf("Error making file executable: %v\n", err)
			os.Exit(1)
		}
	}

	fmt.Printf("✓ Installed to: %s\n", installPath)

	// Offer to add to PATH
	if askYesNo("Add to PATH so you can run 'noteflow' from anywhere?") {
		if err := addToPath(installDir); err != nil {
			fmt.Printf("Warning: Could not add to PATH: %v\n", err)
			fmt.Printf("You can manually add %s to your PATH\n", installDir)
		} else {
			fmt.Println("✓ Added to PATH")
			if runtime.GOOS == "windows" {
				fmt.Println("  Please restart your terminal/PowerShell for PATH changes to take effect")
			} else {
				fmt.Println("  You may need to restart your terminal or run: source ~/.bashrc")
			}
		}
	}

	fmt.Println()
	fmt.Println("Installation complete!")
	fmt.Printf("Run 'noteflow' from any directory to start the application\n")
	fmt.Printf("Or run directly: %s\n", installPath)
}

func getLatestRelease() (*Release, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", repoOwner, repoName)
	
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var release Release
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}

	return &release, nil
}

func getBinaryName() string {
	switch runtime.GOOS {
	case "windows":
		if runtime.GOARCH == "amd64" {
			return "noteflow-go-windows-amd64.exe"
		}
		return fmt.Sprintf("noteflow-go-windows-%s.exe", runtime.GOARCH)
	case "darwin":
		if runtime.GOARCH == "amd64" {
			return "noteflow-go-darwin-amd64"
		}
		return fmt.Sprintf("noteflow-go-darwin-%s", runtime.GOARCH)
	case "linux":
		if runtime.GOARCH == "amd64" {
			return "noteflow-go-linux-amd64"
		}
		return fmt.Sprintf("noteflow-go-linux-%s", runtime.GOARCH)
	default:
		return fmt.Sprintf("noteflow-go-%s-%s", runtime.GOOS, runtime.GOARCH)
	}
}

func getExecutableName() string {
	if runtime.GOOS == "windows" {
		return "noteflow.exe"
	}
	return "noteflow"
}

func getInstallDirectory() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Printf("Error getting home directory: %v\n", err)
		os.Exit(1)
	}

	// Default suggestions based on platform
	var suggestions []string
	switch runtime.GOOS {
	case "windows":
		suggestions = []string{
			filepath.Join(homeDir, "bin"),
			filepath.Join(homeDir, "tools"),
			filepath.Join(homeDir, "Apps"),
			filepath.Join(homeDir, "Desktop"),
		}
	default:
		suggestions = []string{
			filepath.Join(homeDir, "bin"),
			filepath.Join(homeDir, ".local", "bin"),
			filepath.Join(homeDir, "tools"),
			"/usr/local/bin", // Might require sudo
		}
	}

	fmt.Println("Choose installation directory:")
	for i, dir := range suggestions {
		status := ""
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			status = " (will be created)"
		} else if !isWritable(dir) {
			status = " (not writable)"
		}
		fmt.Printf("%d. %s%s\n", i+1, dir, status)
		if i == 0 {
			fmt.Print("   (recommended)")
		}
		fmt.Println()
	}
	fmt.Printf("%d. Custom path\n\n", len(suggestions)+1)

	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("Enter choice (1-" + strconv.Itoa(len(suggestions)+1) + "): ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		choice, err := strconv.Atoi(input)
		if err != nil {
			fmt.Println("Please enter a valid number")
			continue
		}

		if choice >= 1 && choice <= len(suggestions) {
			return suggestions[choice-1]
		} else if choice == len(suggestions)+1 {
			fmt.Print("Enter custom path: ")
			customPath, _ := reader.ReadString('\n')
			return strings.TrimSpace(customPath)
		} else {
			fmt.Printf("Please enter a number between 1 and %d\n", len(suggestions)+1)
		}
	}
}

func isWritable(path string) bool {
	// Try to create a temporary file
	testFile := filepath.Join(path, ".write_test")
	f, err := os.Create(testFile)
	if err != nil {
		return false
	}
	f.Close()
	os.Remove(testFile)
	return true
}

func downloadFile(url, filepath string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func askYesNo(question string) bool {
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Printf("%s (y/n): ", question)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(strings.ToLower(input))
		
		if input == "y" || input == "yes" {
			return true
		} else if input == "n" || input == "no" {
			return false
		} else {
			fmt.Println("Please enter 'y' or 'n'")
		}
	}
}

func addToPath(dir string) error {
	switch runtime.GOOS {
	case "windows":
		return addToWindowsPath(dir)
	default:
		return addToUnixPath(dir)
	}
}

func addToWindowsPath(dir string) error {
	// Get current user PATH
	cmd := fmt.Sprintf(`$path = [Environment]::GetEnvironmentVariable("PATH", "User"); if ($path -notlike "*%s*") { [Environment]::SetEnvironmentVariable("PATH", "$path;%s", "User") }`, dir, dir)
	
	// Use PowerShell to modify user PATH
	return runCommand("powershell", "-Command", cmd)
}

func addToUnixPath(dir string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	// Try to add to common shell config files
	configFiles := []string{
		filepath.Join(homeDir, ".bashrc"),
		filepath.Join(homeDir, ".bash_profile"),
		filepath.Join(homeDir, ".zshrc"),
		filepath.Join(homeDir, ".profile"),
	}

	pathLine := fmt.Sprintf("export PATH=\"$PATH:%s\"", dir)
	
	for _, configFile := range configFiles {
		if _, err := os.Stat(configFile); err == nil {
			// File exists, check if PATH is already added
			content, err := os.ReadFile(configFile)
			if err != nil {
				continue
			}
			
			if strings.Contains(string(content), dir) {
				// Already in PATH
				return nil
			}
			
			// Append to file
			f, err := os.OpenFile(configFile, os.O_APPEND|os.O_WRONLY, 0644)
			if err != nil {
				continue
			}
			defer f.Close()
			
			_, err = f.WriteString("\n# Added by NoteFlow-Go installer\n" + pathLine + "\n")
			return err
		}
	}
	
	// If no config file exists, create .profile
	profilePath := filepath.Join(homeDir, ".profile")
	f, err := os.Create(profilePath)
	if err != nil {
		return err
	}
	defer f.Close()
	
	_, err = f.WriteString("# Added by NoteFlow-Go installer\n" + pathLine + "\n")
	return err
}

func runCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	return cmd.Run()
}