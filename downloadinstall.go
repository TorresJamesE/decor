package main

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// InstallationStatus represents the status of a language installation
type InstallationStatus struct {
	Language      string
	Installed     bool
	Version       string
	LatestVersion string
	Error         string
}

// DownloadInstallModel manages the installation flow
type DownloadInstallModel struct {
	Decor
	selectedLanguages  []string
	installationStatus map[string]*InstallationStatus
	currentIndex       int
	state              string            // "checking", "prompting", "installing", "complete"
	userChoices        map[string]string // "skip" or "install" or "update"
}

// NewDownloadInstallModel creates a new download/install model
func NewDownloadInstallModel(selectedLanguages []string) DownloadInstallModel {
	return DownloadInstallModel{
		selectedLanguages:  selectedLanguages,
		installationStatus: make(map[string]*InstallationStatus),
		userChoices:        make(map[string]string),
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
					return m, installSelectedLanguages(m.selectedLanguages, m.userChoices, m.installationStatus)
				}
			}
		case "n":
			if m.state == "prompting" {
				m.userChoices[m.selectedLanguages[m.currentIndex]] = "skip"
				m.currentIndex++
				if m.currentIndex >= len(m.selectedLanguages) {
					m.state = "installing"
					return m, installSelectedLanguages(m.selectedLanguages, m.userChoices, m.installationStatus)
				}
			}
		case "u":
			if m.state == "prompting" {
				m.userChoices[m.selectedLanguages[m.currentIndex]] = "update"
				m.currentIndex++
				if m.currentIndex >= len(m.selectedLanguages) {
					m.state = "installing"
					return m, installSelectedLanguages(m.selectedLanguages, m.userChoices, m.installationStatus)
				}
			}
		}
	case InstallationStatusMsg:
		m.installationStatus = msg.Status
		m.state = "prompting"
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
		if m.currentIndex >= len(m.selectedLanguages) {
			return ""
		}
		lang := m.selectedLanguages[m.currentIndex]
		status := m.installationStatus[lang]
		return formatPrompt(lang, status)
	case "installing":
		return "Installing selected languages...\n"
	case "complete":
		return "Installation complete!\n"
	default:
		return ""
	}
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

// getLatestVersion gets the latest version of a language (simplified)
func getLatestVersion(language string) string {
	// This is a placeholder - in production, you'd fetch from official sources
	latestVersions := map[string]string{
		"go":     "1.25.5",
		"python": "3.13.0",
		"rust":   "1.81.0",
		"c++":    "14",
		"java":   "21",
	}
	return latestVersions[strings.ToLower(language)]
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

// installSelectedLanguages installs the selected languages
func installSelectedLanguages(languages []string, choices map[string]string, status map[string]*InstallationStatus) tea.Cmd {
	return func() tea.Msg {
		results := make(map[string]string)
		for _, lang := range languages {
			choice := choices[lang]
			switch choice {
			case "skip":
				results[lang] = "skipped"
			case "install":
				err := installLanguage(lang)
				if err != nil {
					results[lang] = fmt.Sprintf("error: %v", err)
				} else {
					results[lang] = "installed"
				}
			case "update":
				err := updateLanguage(lang)
				if err != nil {
					results[lang] = fmt.Sprintf("error: %v", err)
				} else {
					results[lang] = "updated"
				}
			}
		}
		return InstallCompleteMsg{Results: results}
	}
}

// installLanguage downloads and installs a language
func installLanguage(language string) error {
	switch strings.ToLower(language) {
	case "go":
		return installGo()
	case "python":
		return installPython()
	case "rust":
		return installRust()
	case "c++":
		return installCpp()
	case "java":
		return installJava()
	default:
		return fmt.Errorf("unsupported language: %s", language)
	}
}

// updateLanguage updates an existing language installation
func updateLanguage(language string) error {
	// Implementation depends on package manager available on the system
	switch strings.ToLower(language) {
	case "go":
		return updateGo()
	case "python":
		return updatePython()
	case "rust":
		return updateRust()
	case "c++":
		return updateCpp()
	case "java":
		return updateJava()
	default:
		return fmt.Errorf("unsupported language: %s", language)
	}
}

// Language-specific install functions
func installGo() error {
	cmd := exec.Command("bash", "-c", "curl -L https://go.dev/dl/go1.25.5.darwin-arm64.tar.gz -o go1.25.5.tar.gz && tar -C /usr/local -xzf go1.25.5.tar.gz")
	return cmd.Run()
}

func installPython() error {
	if runtime.GOOS == "darwin" {
		cmd := exec.Command("brew", "install", "python@3.13")
		return cmd.Run()
	}
	cmd := exec.Command("apt-get", "install", "-y", "python3")
	return cmd.Run()
}

func installRust() error {
	cmd := exec.Command("bash", "-c", "curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y")
	return cmd.Run()
}

func installCpp() error {
	if runtime.GOOS == "darwin" {
		cmd := exec.Command("xcode-select", "--install")
		return cmd.Run()
	}
	cmd := exec.Command("apt-get", "install", "-y", "build-essential")
	return cmd.Run()
}

func installJava() error {
	if runtime.GOOS == "darwin" {
		cmd := exec.Command("brew", "install", "openjdk@21")
		return cmd.Run()
	}
	cmd := exec.Command("apt-get", "install", "-y", "openjdk-21-jdk")
	return cmd.Run()
}

// Language-specific update functions
func updateGo() error {
	return installGo() // Go update is similar to install
}

func updatePython() error {
	if runtime.GOOS == "darwin" {
		cmd := exec.Command("brew", "upgrade", "python@3.13")
		return cmd.Run()
	}
	cmd := exec.Command("apt-get", "upgrade", "-y", "python3")
	return cmd.Run()
}

func updateRust() error {
	cmd := exec.Command("rustup", "update")
	return cmd.Run()
}

func updateCpp() error {
	if runtime.GOOS == "darwin" {
		cmd := exec.Command("softwareupdate", "-i", "-a")
		return cmd.Run()
	}
	cmd := exec.Command("apt-get", "upgrade", "-y")
	return cmd.Run()
}

func updateJava() error {
	if runtime.GOOS == "darwin" {
		cmd := exec.Command("brew", "upgrade", "openjdk@21")
		return cmd.Run()
	}
	cmd := exec.Command("apt-get", "upgrade", "-y", "openjdk-21-jdk")
	return cmd.Run()
}
