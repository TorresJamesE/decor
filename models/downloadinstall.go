package models

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// InstallationStatus represents the status of a language installation
type InstallationStatus struct {
	Language      string
	Installed     bool
	Version       string
	LatestVersion string
	Error         string
}

// LanguageProgress tracks download/install progress for a language
type LanguageProgress struct {
	Language       string
	Progress       float64 // 0.0 to 1.0
	CurrentStep    string  // "downloading", "installing", "complete", "error"
	TotalSteps     int
	CurrentStepNum int
	ErrorMessage   string
	mu             sync.Mutex
}

// ProgressUpdateMsg is sent when progress changes
type ProgressUpdateMsg struct {
	Language string
	Progress float64
	Step     string
}

// DownloadInstallModel manages the installation flow
type DownloadInstallModel struct {
	Decor
	selectedLanguages  []string
	installationStatus map[string]*InstallationStatus
	currentIndex       int
	state              string            // "checking", "prompting", "installing", "complete"
	userChoices        map[string]string // "skip" or "install" or "update"
	languageProgress   map[string]*LanguageProgress
}

// NewDownloadInstallModel creates a new download/install model
func NewDownloadInstallModel(selectedLanguages []string) DownloadInstallModel {
	return DownloadInstallModel{
		selectedLanguages:  selectedLanguages,
		installationStatus: make(map[string]*InstallationStatus),
		userChoices:        make(map[string]string),
		languageProgress:   make(map[string]*LanguageProgress),
		state:              "checking",
	}
}

func (m DownloadInstallModel) Init() tea.Cmd {
	return tea.Batch(
		checkInstalledLanguages(m.selectedLanguages),
	)
}

func (m DownloadInstallModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "y", "enter":
			if m.state == "prompting" {
				m.userChoices[m.selectedLanguages[m.currentIndex]] = getDefaultChoice(m.installationStatus[m.selectedLanguages[m.currentIndex]])
				m.currentIndex++
				if m.currentIndex >= len(m.selectedLanguages) {
					m.state = "installing"
					return m, installSelectedLanguagesWithProgress(m.selectedLanguages, m.userChoices, m.installationStatus)
				}
			}
		case "n":
			if m.state == "prompting" {
				m.userChoices[m.selectedLanguages[m.currentIndex]] = "skip"
				m.currentIndex++
				if m.currentIndex >= len(m.selectedLanguages) {
					m.state = "installing"
					return m, installSelectedLanguagesWithProgress(m.selectedLanguages, m.userChoices, m.installationStatus)
				}
			}
		case "u":
			if m.state == "prompting" {
				m.userChoices[m.selectedLanguages[m.currentIndex]] = "update"
				m.currentIndex++
				if m.currentIndex >= len(m.selectedLanguages) {
					m.state = "installing"
					return m, installSelectedLanguagesWithProgress(m.selectedLanguages, m.userChoices, m.installationStatus)
				}
			}
		}
	case InstallationStatusMsg:
		m.installationStatus = msg.Status
		m.state = "prompting"
	case InitProgressMsg:
		m.languageProgress = msg.Trackers
		return m, progressUpdateTicker()
	case ProgressTickMsg:
		// Check if any language is still installing
		allComplete := true
		for _, prog := range m.languageProgress {
			prog.mu.Lock()
			if prog.Progress < 1.0 && prog.CurrentStep != "error" {
				allComplete = false
			}
			prog.mu.Unlock()
		}
		if allComplete {
			m.state = "complete"
			return m, nil
		}
		return m, progressUpdateTicker()
	case ProgressUpdateMsg:
		if progress, exists := m.languageProgress[msg.Language]; exists {
			progress.mu.Lock()
			progress.Progress = msg.Progress
			progress.CurrentStep = msg.Step
			progress.mu.Unlock()
		}
		return m, progressUpdateTicker()
	case InstallCompleteMsg:
		m.state = "complete"
		return m, nil
	case InstallErrorMsg:
		return m, nil
	}
	return m, nil
}

func (m DownloadInstallModel) View() string {
	switch m.state {
	case "checking":
		return "Checking installed languages...\n"
	case "prompting":
		var output string

		// Show all checked languages and their status
		output += "\n=== Installation Status ===\n"
		for _, lang := range m.selectedLanguages {
			status := m.installationStatus[lang]
			if status == nil {
				continue
			}
			output += formatStatusLine(lang, status)
		}

		output += "\n"

		// Show the current prompt
		if m.currentIndex >= len(m.selectedLanguages) {
			return output
		}
		lang := m.selectedLanguages[m.currentIndex]
		status := m.installationStatus[lang]
		output += formatPrompt(lang, status)
		return output
	case "installing":
		return m.renderInstallationProgress()
	case "complete":
		var output string
		output += "\n=== Installation Complete ===\n"
		for lang, result := range m.userChoices {
			output += fmt.Sprintf("%s: %s\n", lang, result)
		}
		return output
	default:
		return ""
	}
}

// renderInstallationProgress renders styled progress bars for all languages
func (m DownloadInstallModel) renderInstallationProgress() string {
	// Define lipgloss styles
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("11")). // Yellow
		MarginBottom(1)

	progressContainerStyle := lipgloss.NewStyle().
		MarginBottom(1).
		PaddingLeft(2)

	langNameStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("6")). // Cyan
		Width(15)

	progressBarStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("10")). // Green
		MarginLeft(1)

	statusStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("8")). // Gray
		MarginLeft(1)

	var output string
	output += titleStyle.Render("Installing Languages...") + "\n"

	for _, lang := range m.selectedLanguages {
		choice := m.userChoices[lang]
		if choice == "skip" {
			output += progressContainerStyle.Render(
				lipgloss.JoinHorizontal(
					lipgloss.Left,
					langNameStyle.Render(lang),
					statusStyle.Render("⊘ Skipped"),
				),
			) + "\n"
			continue
		}

		// Initialize progress if not exists
		if _, exists := m.languageProgress[lang]; !exists {
			m.languageProgress[lang] = &LanguageProgress{
				Language:    lang,
				Progress:    0.0,
				CurrentStep: "starting",
				TotalSteps:  3,
			}
		}

		prog := m.languageProgress[lang]
		prog.mu.Lock()
		progress := prog.Progress
		step := prog.CurrentStep
		prog.mu.Unlock()

		// Create a progress modal
		progressBar := renderProgressBar(progress, 30)

		output += progressContainerStyle.Render(
			lipgloss.JoinHorizontal(
				lipgloss.Left,
				langNameStyle.Render(lang),
				progressBarStyle.Render(progressBar),
				statusStyle.Render(fmt.Sprintf("(%s)", step)),
			),
		) + "\n"
	}

	return output
}

// renderProgressBar creates a visual progress bar with percentage
func renderProgressBar(progress float64, width int) string {
	filled := int(float64(width) * progress)
	if filled > width {
		filled = width
	}
	empty := width - filled

	// Create the bar
	bar := "[" + strings.Repeat("=", filled) + strings.Repeat(" ", empty) + "]"

	// Add percentage
	percentage := fmt.Sprintf("%.0f%%", progress*100)

	return bar + " " + percentage
}

// Message types for async operations
type InstallationStatusMsg struct {
	Status map[string]*InstallationStatus
}

type InstallCompleteMsg struct {
	Results map[string]string
}

type InstallErrorMsg struct {
	Language string
	Error    string
}

// progressUpdateTicker sends periodic progress updates
func progressUpdateTicker() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(time.Time) tea.Msg {
		return ProgressTickMsg{}
	})
}

type ProgressTickMsg struct{}

// checkInstalledLanguages checks which languages are installed
func checkInstalledLanguages(languages []string) tea.Cmd {
	return func() tea.Msg {
		status := make(map[string]*InstallationStatus)
		for _, lang := range languages {
			installed, version, latest := checkLanguageInstallation(lang)
			status[lang] = &InstallationStatus{
				Language:      lang,
				Installed:     installed,
				Version:       version,
				LatestVersion: latest,
			}
		}
		return InstallationStatusMsg{Status: status}
	}
}

// checkLanguageInstallation checks if a language is installed and gets its version
func checkLanguageInstallation(language string) (bool, string, string) {
	var cmd *exec.Cmd
	switch strings.ToLower(language) {
	case "go":
		cmd = exec.Command("go", "version")
	case "python":
		cmd = exec.Command("python3", "--version")
	case "rust":
		cmd = exec.Command("rustc", "--version")
	case "c++":
		if runtime.GOOS == "darwin" {
			cmd = exec.Command("clang", "--version")
		} else {
			cmd = exec.Command("g++", "--version")
		}
	case "java":
		cmd = exec.Command("java", "-version")
	default:
		return false, "", ""
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return false, "", ""
	}

	version := parseVersion(string(output), language)
	latest := getLatestVersion(language)

	return true, version, latest
}

// parseVersion extracts version from command output
func parseVersion(output, language string) string {
	lines := strings.Split(output, "\n")
	if len(lines) > 0 {
		return strings.TrimSpace(lines[0])
	}
	return "unknown"
}

func createSecureClient() *http.Client {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12, // Minimum TLS 1.2
		},
		DisableCompression: false,
		MaxIdleConns:       100,
		IdleConnTimeout:    90 * time.Second,
	}

	return &http.Client{
		Transport: transport,
		Timeout:   5 * time.Second,
	}
}

// getLatestVersion gets the latest version of a language (simplified)
func getLatestVersion(language string) string {

	latestVersions := map[string]string{
		"go":     "1.25.5",
		"python": "3.13.0",
		"rust":   "1.81.0",
		"c++":    "14",
		"java":   "21",
	}
	return latestVersions[strings.ToLower(language)]
}

// formatStatusLine formats the installation status for display
func formatStatusLine(language string, status *InstallationStatus) string {
	if !status.Installed {
		return fmt.Sprintf("  ❌ %s: NOT INSTALLED\n", language)
	}

	if status.Version == status.LatestVersion {
		return fmt.Sprintf("  ✅ %s: %s (latest)\n", language, status.Version)
	}

	return fmt.Sprintf("  ⚠️  %s: %s (latest: %s)\n", language, status.Version, status.LatestVersion)
}

// formatPrompt formats the installation prompt for the user
func formatPrompt(language string, status *InstallationStatus) string {
	if !status.Installed {
		return fmt.Sprintf(
			"%s is not installed.\n(i) Install\n(s) Skip\n",
			language,
		)
	}

	if status.Version == status.LatestVersion {
		return fmt.Sprintf(
			"%s is installed (version: %s).\n(s) Skip\n(r) Reinstall\n",
			language,
			status.Version,
		)
	}

	return fmt.Sprintf(
		"%s is installed (current: %s, latest: %s).\n(u) Update\n(s) Skip\n",
		language,
		status.Version,
		status.LatestVersion,
	)
}

// getDefaultChoice returns the default choice based on installation status
func getDefaultChoice(status *InstallationStatus) string {
	if !status.Installed {
		return "install"
	}
	if status.Version != status.LatestVersion {
		return "update"
	}
	return "skip"
}

// installSelectedLanguagesWithProgress installs languages with progress tracking
func installSelectedLanguagesWithProgress(languages []string, choices map[string]string, status map[string]*InstallationStatus) tea.Cmd {
	return tea.Batch(
		func() tea.Msg {
			// Create and initialize progress trackers for all non-skipped languages
			progressTrackers := make(map[string]*LanguageProgress)
			for _, lang := range languages {
				choice := choices[lang]
				if choice != "skip" {
					progressTrackers[lang] = &LanguageProgress{
						Language:    lang,
						Progress:    0.0,
						CurrentStep: "starting",
						TotalSteps:  3,
					}
				}
			}

			// Start installation in background
			go func() {
				results := make(map[string]string)
				var wg sync.WaitGroup

				for _, lang := range languages {
					choice := choices[lang]
					if choice == "skip" {
						results[lang] = "skipped"
						continue
					}

					progress := progressTrackers[lang]

					wg.Add(1)
					go func(language, choiceType string, prog *LanguageProgress) {
						defer wg.Done()
						var err error

						switch choiceType {
						case "install":
							err = installLanguageWithProgress(language, prog)
							if err != nil {
								results[language] = fmt.Sprintf("error: %v", err)
								prog.CurrentStep = "error"
								prog.ErrorMessage = err.Error()
							} else {
								results[language] = "installed"
								prog.CurrentStep = "complete"
								prog.Progress = 1.0
							}
						case "update":
							err = updateLanguageWithProgress(language, prog)
							if err != nil {
								results[language] = fmt.Sprintf("error: %v", err)
								prog.CurrentStep = "error"
								prog.ErrorMessage = err.Error()
							} else {
								results[language] = "updated"
								prog.CurrentStep = "complete"
								prog.Progress = 1.0
							}
						}
					}(lang, choice, progress)
				}

				wg.Wait()
				// Send completion message (handled by completion ticker)
			}()

			// Store trackers in a shared location
			return InitProgressMsg{Trackers: progressTrackers}
		},
		progressUpdateTicker(),
	)
}

// InitProgressMsg initializes progress trackers
type InitProgressMsg struct {
	Trackers map[string]*LanguageProgress
}

// installLanguageWithProgress downloads and installs a language with progress tracking
func installLanguageWithProgress(language string, progress *LanguageProgress) error {
	switch strings.ToLower(language) {
	case "go":
		return installGoWithProgress(progress)
	case "python":
		return installPythonWithProgress(progress)
	case "rust":
		return installRustWithProgress(progress)
	case "c++":
		return installCppWithProgress(progress)
	case "java":
		return installJavaWithProgress(progress)
	default:
		return fmt.Errorf("unsupported language: %s", language)
	}
}

// updateLanguageWithProgress updates an existing language installation with progress
func updateLanguageWithProgress(language string, progress *LanguageProgress) error {
	switch strings.ToLower(language) {
	case "go":
		return updateGoWithProgress(progress)
	case "python":
		return updatePythonWithProgress(progress)
	case "rust":
		return updateRustWithProgress(progress)
	case "c++":
		return updateCppWithProgress(progress)
	case "java":
		return updateJavaWithProgress(progress)
	default:
		return fmt.Errorf("unsupported language: %s", language)
	}
}

// Language-specific install functions with progress tracking
func installGoWithProgress(progress *LanguageProgress) error {
	steps := []string{
		"Downloading Go...",
		"Extracting files...",
		"Verifying installation...",
	}

	for i, step := range steps {
		progress.mu.Lock()
		progress.Progress = float64(i) / float64(len(steps))
		progress.CurrentStep = step
		progress.mu.Unlock()
		time.Sleep(500 * time.Millisecond) // Simulate work
	}

	progress.mu.Lock()
	progress.Progress = 1.0
	progress.CurrentStep = "Verifying installation..."
	progress.mu.Unlock()

	cmd := exec.Command("bash", "-c", "curl -L https://go.dev/dl/go1.25.5.darwin-arm64.tar.gz -o go1.25.5.tar.gz && tar -C /usr/local -xzf go1.25.5.tar.gz")
	return cmd.Run()
}

func installPythonWithProgress(progress *LanguageProgress) error {
	steps := []string{
		"Preparing installation...",
		"Installing Python...",
		"Verifying installation...",
	}

	for i, step := range steps {
		progress.mu.Lock()
		progress.Progress = float64(i) / float64(len(steps))
		progress.CurrentStep = step
		progress.mu.Unlock()
		time.Sleep(500 * time.Millisecond) // Simulate work
	}

	progress.mu.Lock()
	progress.Progress = 1.0
	progress.CurrentStep = "Verifying installation..."
	progress.mu.Unlock()

	if runtime.GOOS == "darwin" {
		fmt.Println("Installing Python using Homebrew...")
		cmd := exec.Command("brew", "install", "python@3.13")
		return cmd.Run()
	}
	cmd := exec.Command("apt-get", "install", "-y", "python3")
	return cmd.Run()
}

func installRustWithProgress(progress *LanguageProgress) error {
	steps := []string{
		"Downloading Rust installer...",
		"Running installation script...",
		"Configuring environment...",
	}

	for i, step := range steps {
		progress.mu.Lock()
		progress.Progress = float64(i) / float64(len(steps))
		progress.CurrentStep = step
		progress.mu.Unlock()
		time.Sleep(500 * time.Millisecond) // Simulate work
	}

	progress.mu.Lock()
	progress.Progress = 1.0
	progress.CurrentStep = "Configuring environment..."
	progress.mu.Unlock()

	cmd := exec.Command("bash", "-c", "curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y")
	return cmd.Run()
}

func installCppWithProgress(progress *LanguageProgress) error {
	steps := []string{
		"Preparing installation...",
		"Installing C++ compiler...",
		"Setting up environment...",
	}

	for i, step := range steps {
		progress.mu.Lock()
		progress.Progress = float64(i) / float64(len(steps))
		progress.CurrentStep = step
		progress.mu.Unlock()
		time.Sleep(500 * time.Millisecond) // Simulate work
	}

	progress.mu.Lock()
	progress.Progress = 1.0
	progress.CurrentStep = "Setting up environment..."
	progress.mu.Unlock()

	if runtime.GOOS == "darwin" {
		cmd := exec.Command("xcode-select", "--install")
		return cmd.Run()
	}
	cmd := exec.Command("apt-get", "install", "-y", "build-essential")
	return cmd.Run()
}

func installJavaWithProgress(progress *LanguageProgress) error {
	steps := []string{
		"Preparing installation...",
		"Installing OpenJDK...",
		"Setting up environment...",
	}

	for i, step := range steps {
		progress.mu.Lock()
		progress.Progress = float64(i) / float64(len(steps))
		progress.CurrentStep = step
		progress.mu.Unlock()
		time.Sleep(500 * time.Millisecond) // Simulate work
	}

	progress.mu.Lock()
	progress.Progress = 1.0
	progress.CurrentStep = "Setting up environment..."
	progress.mu.Unlock()

	if runtime.GOOS == "darwin" {
		cmd := exec.Command("brew", "install", "openjdk@21")
		return cmd.Run()
	}
	cmd := exec.Command("apt-get", "install", "-y", "openjdk-21-jdk")
	return cmd.Run()
}

// Language-specific update functions with progress tracking
func updateGoWithProgress(progress *LanguageProgress) error {
	steps := []string{
		"Checking latest version...",
		"Downloading Go...",
		"Installing update...",
	}

	for i, step := range steps {
		progress.mu.Lock()
		progress.Progress = float64(i) / float64(len(steps))
		progress.CurrentStep = step
		progress.mu.Unlock()
		time.Sleep(500 * time.Millisecond) // Simulate work
	}

	progress.mu.Lock()
	progress.Progress = 1.0
	progress.CurrentStep = "Installing update..."
	progress.mu.Unlock()

	return installGoWithProgress(progress)
}

func updatePythonWithProgress(progress *LanguageProgress) error {
	steps := []string{
		"Fetching available updates...",
		"Upgrading Python...",
		"Verifying update...",
	}

	for i, step := range steps {
		progress.mu.Lock()
		progress.Progress = float64(i) / float64(len(steps))
		progress.CurrentStep = step
		progress.mu.Unlock()
		time.Sleep(500 * time.Millisecond) // Simulate work
	}

	progress.mu.Lock()
	progress.Progress = 1.0
	progress.CurrentStep = "Verifying update..."
	progress.mu.Unlock()

	if runtime.GOOS == "darwin" {
		cmd := exec.Command("brew", "upgrade", "python@3.13")
		return cmd.Run()
	}
	cmd := exec.Command("apt-get", "upgrade", "-y", "python3")
	return cmd.Run()
}

func updateRustWithProgress(progress *LanguageProgress) error {
	steps := []string{
		"Checking for updates...",
		"Updating Rust...",
		"Verifying update...",
	}

	for i, step := range steps {
		progress.mu.Lock()
		progress.Progress = float64(i) / float64(len(steps))
		progress.CurrentStep = step
		progress.mu.Unlock()
		time.Sleep(500 * time.Millisecond) // Simulate work
	}

	progress.mu.Lock()
	progress.Progress = 1.0
	progress.CurrentStep = "Verifying update..."
	progress.mu.Unlock()

	cmd := exec.Command("rustup", "update")
	return cmd.Run()
}

func updateCppWithProgress(progress *LanguageProgress) error {
	steps := []string{
		"Checking for system updates...",
		"Installing updates...",
		"Verifying...",
	}

	for i, step := range steps {
		progress.mu.Lock()
		progress.Progress = float64(i) / float64(len(steps))
		progress.CurrentStep = step
		progress.mu.Unlock()
		time.Sleep(500 * time.Millisecond) // Simulate work
	}

	progress.mu.Lock()
	progress.Progress = 1.0
	progress.CurrentStep = "Verifying..."
	progress.mu.Unlock()

	if runtime.GOOS == "darwin" {
		cmd := exec.Command("softwareupdate", "-i", "-a")
		return cmd.Run()
	}
	cmd := exec.Command("apt-get", "upgrade", "-y")
	return cmd.Run()
}

func updateJavaWithProgress(progress *LanguageProgress) error {
	steps := []string{
		"Fetching available updates...",
		"Upgrading OpenJDK...",
		"Verifying update...",
	}

	for i, step := range steps {
		progress.mu.Lock()
		progress.Progress = float64(i) / float64(len(steps))
		progress.CurrentStep = step
		progress.mu.Unlock()
		time.Sleep(500 * time.Millisecond) // Simulate work
	}

	progress.mu.Lock()
	progress.Progress = 1.0
	progress.CurrentStep = "Verifying update..."
	progress.mu.Unlock()

	if runtime.GOOS == "darwin" {
		cmd := exec.Command("brew", "upgrade", "openjdk@21")
		return cmd.Run()
	}
	cmd := exec.Command("apt-get", "upgrade", "-y", "openjdk-21-jdk")
	return cmd.Run()
}
