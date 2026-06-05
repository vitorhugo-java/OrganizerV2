//go:build windows

package notifier

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/go-toast/toast"
	"github.com/vitorhugo-java/organizerv2/internal/config"
	"github.com/vitorhugo-java/organizerv2/internal/pathutil"
	"golang.design/x/clipboard"
)

type windowsNotifier struct {
	cfg           config.NotificationConfig
	clipboardInit bool
}

func newPlatform(cfg config.NotificationConfig) Notifier {
	n := &windowsNotifier{cfg: cfg}
	if cfg.Actions.CopyPath {
		if err := clipboard.Init(); err != nil {
			log.Printf("[notifier] clipboard init failed: %v", err)
		} else {
			n.clipboardInit = true
		}
	}
	return n
}

func (n *windowsNotifier) Notify(event FileEvent) error {
	go n.deliver(event)
	return nil
}

func (n *windowsNotifier) deliver(event FileEvent) {
	filename := filepath.Base(event.Destination)

	// Simple informational toast (does not require COM activation).
	notification := toast.Notification{
		AppID:   "OrganizerV2",
		Title:   "OrganizerV2",
		Message: fmt.Sprintf("%s → %s/", filename, event.Category),
	}
	if err := notification.Push(); err != nil {
		log.Printf("[notifier] toast push error: %v", err)
	}

	// Legacy path-to-clipboard copy.
	if n.cfg.Actions.CopyPath && n.clipboardInit {
		clipboard.Write(clipboard.FmtText, []byte(event.Destination))
	}

	// Interactive WinForms dialog for destination selection and actions.
	action, dest, err := showDestinationDialog(n.cfg, event)
	if err != nil {
		log.Printf("[notifier] dialog error: %v", err)
		return
	}

	switch action {
	case "open_file":
		openWithShell(dest)
	case "open_location":
		openLocation(dest)
	case "move":
		if err := redirectFile(event.Destination, dest, false); err != nil {
			log.Printf("[notifier] move to %s error: %v", dest, err)
		}
	case "copy":
		if err := redirectFile(event.Destination, dest, true); err != nil {
			log.Printf("[notifier] copy to %s error: %v", dest, err)
		}
	// "keep" and "" -> nothing to do.
	}
}

// redirectFile moves or copies srcFile into destDir, resolving duplicate
// filenames so no existing file is overwritten.
func redirectFile(srcFile, destDir string, copyOnly bool) error {
	if err := pathutil.EnsureDir(destDir); err != nil {
		return fmt.Errorf("mkdir %s: %w", destDir, err)
	}
	raw := filepath.Join(destDir, filepath.Base(srcFile))
	dest, err := pathutil.ResolveDuplicate(raw)
	if err != nil {
		return err
	}
	if copyOnly {
		return pathutil.CopyFile(srcFile, dest)
	}
	return pathutil.MoveFile(srcFile, dest)
}

// showDestinationDialog writes a temporary PowerShell script, executes it to
// display a native Windows Forms dialog, and returns the user's chosen action
// and destination path.  Returns ("keep", "", nil) on dismissal or timeout.
func showDestinationDialog(cfg config.NotificationConfig, event FileEvent) (action, dest string, err error) {
	var names, paths []string
	for _, s := range cfg.Shortcuts {
		names = append(names, s.Name)
		paths = append(paths, s.Path)
	}

	tmp, err := os.CreateTemp("", "organizer_dialog_*.ps1")
	if err != nil {
		return "keep", "", fmt.Errorf("temp script: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if _, err := tmp.WriteString(dialogScript); err != nil {
		tmp.Close()
		return "keep", "", err
	}
	tmp.Close()

	args := []string{
		"-NoProfile", "-NonInteractive",
		"-ExecutionPolicy", "Bypass",
		"-File", tmpPath,
		"-CurrentFile", event.Destination,
		"-Category", event.Category,
		"-ShortcutNames", strings.Join(names, ";"),
		"-ShortcutPaths", strings.Join(paths, ";"),
	}
	if !cfg.Actions.CopyFile {
		args = append(args, "-NoCopyFile")
	}
	if !cfg.Actions.OpenFile {
		args = append(args, "-NoOpenFile")
	}
	if !cfg.Actions.OpenLocation {
		args = append(args, "-NoOpenLocation")
	}
	if !cfg.Actions.Confirm {
		args = append(args, "-NoConfirm")
	}

	out, _ := exec.Command("powershell", args...).Output()

	// Parse the last non-empty output line: "<action>|<path>"
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var line string
	for i := len(lines) - 1; i >= 0; i-- {
		if l := strings.TrimSpace(lines[i]); l != "" {
			line = l
			break
		}
	}
	if line == "" {
		return "keep", "", nil
	}
	parts := strings.SplitN(line, "|", 2)
	if len(parts) != 2 {
		return "keep", "", nil
	}
	return parts[0], parts[1], nil
}

func openWithShell(path string) {
	cmd := exec.Command("cmd", "/c", "start", "", path)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		log.Printf("[notifier] open error: %v", err)
	}
}

func openLocation(dirPath string) {
	cmd := exec.Command("explorer", dirPath)
	if err := cmd.Start(); err != nil {
		log.Printf("[notifier] explorer error: %v", err)
	}
}

func (n *windowsNotifier) Close() error { return nil }

// dialogScript is the PowerShell WinForms destination-selector dialog.
// It always shows a ComboBox with configured shortcuts plus a "Browse..." option,
// so Move To / Copy To buttons are always available regardless of config.
// Writes one line to stdout: "<action>|<path>"
// where action is one of: keep, open_file, open_location, move, copy.
const dialogScript = `
param(
    [string]$CurrentFile,
    [string]$Category,
    [string]$ShortcutNames = '',
    [string]$ShortcutPaths = '',
    [switch]$NoCopyFile,
    [switch]$NoOpenFile,
    [switch]$NoOpenLocation,
    [switch]$NoConfirm
)

Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName System.Drawing
[System.Windows.Forms.Application]::EnableVisualStyles()

$script:result      = "keep|$CurrentFile"
$script:browsedPath = ''
$filename    = Split-Path $CurrentFile -Leaf
$currentDir  = Split-Path $CurrentFile -Parent

# Build ordered shortcut map.
$destMap = [ordered]@{}
if ($ShortcutNames -ne '') {
    $sNames = $ShortcutNames -split ';'
    $sPaths = $ShortcutPaths -split ';'
    for ($i = 0; $i -lt $sNames.Length; $i++) {
        $n = $sNames[$i].Trim()
        $p = if ($i -lt $sPaths.Length) { $sPaths[$i].Trim() } else { '' }
        if ($n -and $p) { $destMap[$n] = $p }
    }
}

$browseKey = 'Browse...'

# Form
$form = New-Object System.Windows.Forms.Form
$form.Text            = 'OrganizerV2'
$form.ClientSize      = New-Object System.Drawing.Size(520, 205)
$form.StartPosition   = 'CenterScreen'
$form.TopMost         = $true
$form.FormBorderStyle = 'FixedDialog'
$form.MaximizeBox     = $false
$form.MinimizeBox     = $false

# Title label
$lblTitle = New-Object System.Windows.Forms.Label
$lblTitle.Text     = "$filename - moved to $Category"
$lblTitle.Font     = New-Object System.Drawing.Font('Segoe UI', 10, [System.Drawing.FontStyle]::Bold)
$lblTitle.Location = New-Object System.Drawing.Point(12, 12)
$lblTitle.Size     = New-Object System.Drawing.Size(496, 24)
$form.Controls.Add($lblTitle)

# Destination path (small grey)
$lblPath = New-Object System.Windows.Forms.Label
$lblPath.Text      = $CurrentFile
$lblPath.Font      = New-Object System.Drawing.Font('Segoe UI', 8)
$lblPath.ForeColor = [System.Drawing.Color]::Gray
$lblPath.Location  = New-Object System.Drawing.Point(12, 38)
$lblPath.Size      = New-Object System.Drawing.Size(496, 16)
$form.Controls.Add($lblPath)

# Separator
$sep = New-Object System.Windows.Forms.Label
$sep.BorderStyle = 'FixedSingle'
$sep.Location    = New-Object System.Drawing.Point(12, 62)
$sep.Size        = New-Object System.Drawing.Size(496, 1)
$form.Controls.Add($sep)

# Redirect to label
$lblDest = New-Object System.Windows.Forms.Label
$lblDest.Text     = 'Redirect to:'
$lblDest.Font     = New-Object System.Drawing.Font('Segoe UI', 9)
$lblDest.Location = New-Object System.Drawing.Point(12, 74)
$lblDest.Size     = New-Object System.Drawing.Size(90, 22)
$form.Controls.Add($lblDest)

# ComboBox - always shown with shortcuts + Browse...
$script:combo = New-Object System.Windows.Forms.ComboBox
$script:combo.DropDownStyle = 'DropDownList'
$script:combo.Font          = New-Object System.Drawing.Font('Segoe UI', 9)
$script:combo.Location      = New-Object System.Drawing.Point(107, 71)
$script:combo.Size          = New-Object System.Drawing.Size(401, 22)
$null = $script:combo.Items.Add("Keep in $Category (current)")
foreach ($key in $destMap.Keys) { $null = $script:combo.Items.Add($key) }
$null = $script:combo.Items.Add($browseKey)
$script:combo.SelectedIndex = 0
$form.Controls.Add($script:combo)

# Button panel
$panel = New-Object System.Windows.Forms.FlowLayoutPanel
$panel.Location      = New-Object System.Drawing.Point(12, 108)
$panel.Size          = New-Object System.Drawing.Size(496, 82)
$panel.FlowDirection = 'LeftToRight'
$panel.WrapContents  = $true

function New-Btn($Text, $Width) {
    $b = New-Object System.Windows.Forms.Button
    $b.Text   = $Text
    $b.Width  = $Width
    $b.Height = 30
    $b.Margin = New-Object System.Windows.Forms.Padding(0, 0, 6, 4)
    return $b
}

if (-not $NoOpenFile) {
    $btnOpenFile = New-Btn 'Open File' 90
    $btnOpenFile.Add_Click({
        $script:result = "open_file|$CurrentFile"
        $form.Close()
    })
    $panel.Controls.Add($btnOpenFile)
}

if (-not $NoOpenLocation) {
    $btnOpenFolder = New-Btn 'Open Folder' 104
    $btnOpenFolder.Add_Click({
        $script:result = "open_location|$currentDir"
        $form.Close()
    })
    $panel.Controls.Add($btnOpenFolder)
}

# Move To button - always shown, enabled when a destination is selected
$script:btnMove = New-Btn 'Move To' 74
$script:btnMove.Enabled = $false
$script:btnMove.Add_Click({
    $sel = $script:combo.SelectedItem.ToString()
    $dest = if ($sel -eq $browseKey) { $script:browsedPath } elseif ($destMap.Contains($sel)) { $destMap[$sel] } else { '' }
    if ($dest) {
        $script:result = "move|$dest"
        $form.Close()
    }
})
$panel.Controls.Add($script:btnMove)

if (-not $NoCopyFile) {
    $script:btnCopy = New-Btn 'Copy To' 74
    $script:btnCopy.Enabled = $false
    $script:btnCopy.Add_Click({
        $sel = $script:combo.SelectedItem.ToString()
        $dest = if ($sel -eq $browseKey) { $script:browsedPath } elseif ($destMap.Contains($sel)) { $destMap[$sel] } else { '' }
        if ($dest) {
            $script:result = "copy|$dest"
            $form.Close()
        }
    })
    $panel.Controls.Add($script:btnCopy)
}

# ComboBox selection handler
$script:combo.Add_SelectedIndexChanged({
    $sel = $script:combo.SelectedItem.ToString()
    if ($sel -eq $browseKey) {
        $dlg = New-Object System.Windows.Forms.FolderBrowserDialog
        $dlg.Description = 'Choose destination folder'
        $dlg.ShowNewFolderButton = $true
        if ($dlg.ShowDialog() -eq 'OK') {
            $script:browsedPath = $dlg.SelectedPath
            $script:btnMove.Enabled = $true
            if ($script:btnCopy) { $script:btnCopy.Enabled = $true }
        } else {
            $script:combo.SelectedIndex = 0
        }
    } else {
        $isShortcut = $script:combo.SelectedIndex -gt 0
        $script:btnMove.Enabled = $isShortcut
        if ($script:btnCopy) { $script:btnCopy.Enabled = $isShortcut }
        $script:browsedPath = ''
    }
})

if (-not $NoConfirm) {
    $btnConfirm = New-Btn 'Confirm' 74
    $btnConfirm.Add_Click({
        $script:result = "keep|$currentDir"
        $form.Close()
    })
    $panel.Controls.Add($btnConfirm)
}

$form.Controls.Add($panel)

# Auto-close after 60 s if the user does not interact.
$timer = New-Object System.Windows.Forms.Timer
$timer.Interval = 60000
$timer.Add_Tick({
    $timer.Stop()
    $script:result = "keep|$currentDir"
    $form.Close()
})
$timer.Start()

[void]$form.ShowDialog()
$timer.Stop()
Write-Output $script:result
`
