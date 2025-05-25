package ui

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jroimartin/gocui"
)

var defaultBtrfsPath string

// SetupPrefixes sets up prefixes for subvolumes and snapshots from environment variables
func SetupPrefixes() {
	if prefix := os.Getenv("SUBVOLUME_PREFIX"); prefix != "" {
		subvolumePrefix = prefix
	}
	if prefix := os.Getenv("SNAPSHOT_PREFIX"); prefix != "" {
		snapshotPrefix = prefix
	}
}

const (
	viewDiskInfo    = "diskInfo"
	viewSubvolumes  = "subvolumes"
	viewSnapshots   = "snapshots"
	viewSnapshotInfo = "snapshotInfo"
	viewHotkeys     = "hotkeys"
	viewDialog      = "dialog"
)

type UI struct {
	gui *gocui.Gui
	currentView string
	subvolumesData *ViewData
	snapshotsData *ViewData
}

func Run(btrfsPath string) error {
	defaultBtrfsPath = btrfsPath
	gui, err := gocui.NewGui(gocui.OutputNormal)
	if err != nil {
		return fmt.Errorf("failed to create gui: %v", err)
	}
	defer gui.Close()

	ui := &UI{
		gui: gui,
		currentView: viewSubvolumes,
		subvolumesData: NewViewData(),
		snapshotsData: NewViewData(),
	}
	gui.SetManager(ui)

	if err := ui.setKeyBindings(); err != nil {
		return fmt.Errorf("failed to set key bindings: %v", err)
	}

	if err := gui.MainLoop(); err != nil && err != gocui.ErrQuit {
		return fmt.Errorf("main loop failed: %v", err)
	}

	return nil
}

func (ui *UI) Layout(gui *gocui.Gui) error {
	maxX, maxY := gui.Size()

	// Disk info view - top
	if v, err := gui.SetView(viewDiskInfo, 0, 0, maxX-1, 2); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Title = "Disk Info"
		v.Frame = true
	}

	// Subvolumes view - left
	if v, err := gui.SetView(viewSubvolumes, 0, 3, (maxX/5)-1, maxY-3); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Title = "Subvolumes"
		v.Highlight = true  // Initial view
		v.SelBgColor = gocui.ColorGreen
		v.SelFgColor = gocui.ColorBlack
		v.Frame = true
		if _, err := ui.gui.SetCurrentView(viewSubvolumes); err != nil {
			return err
		}
		ui.UpdateViewContent()
	}

	// Snapshots view - middle
	if v, err := gui.SetView(viewSnapshots, maxX/5, 3, (maxX*2/4)-1, maxY-3); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Title = "Snapshots"
		v.Highlight = false  // Inactive view
		v.SelBgColor = gocui.ColorGreen
		v.SelFgColor = gocui.ColorBlack
		v.Frame = true
		ui.UpdateViewContent()
	}

	// Snapshot info view - right
	if v, err := gui.SetView(viewSnapshotInfo, (maxX*2/4), 3, maxX-1, maxY-3); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Title = "Snapshot Info"
		v.Wrap = true
		v.Frame = true
		ui.UpdateViewContent()
	}

	// Hotkeys view - bottom
	if v, err := gui.SetView(viewHotkeys, 0, maxY-3, maxX-1, maxY-1); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Title = "Hotkeys"
		v.Frame = false
		fmt.Fprintln(v, "q: Quit | ←/→: Switch view | ↑/↓: Navigate | t: Take snapshot")
		ui.updateHotkeys()
	}

	return nil
}

func (ui *UI) setKeyBindings() error {
	// Quit
	if err := ui.gui.SetKeybinding("", 'q', gocui.ModNone, quit); err != nil {
		return err
	}

	// Create snapshot
	if err := ui.gui.SetKeybinding(viewSubvolumes, 't', gocui.ModNone, ui.createSnapshot); err != nil {
		return err
	}

	// Delete snapshot
	if err := ui.gui.SetKeybinding(viewSnapshots, 'r', gocui.ModNone, ui.deleteSnapshot); err != nil {
		return err
	}

	// Navigation between views
	if err := ui.gui.SetKeybinding("", gocui.KeyArrowLeft, gocui.ModNone, ui.prevView); err != nil {
		return err
	}
	if err := ui.gui.SetKeybinding("", gocui.KeyArrowRight, gocui.ModNone, ui.nextView); err != nil {
		return err
	}

	// Navigation within views
	views := []string{viewSubvolumes, viewSnapshots}
	for _, view := range views {
		if err := ui.gui.SetKeybinding(view, gocui.KeyArrowUp, gocui.ModNone, ui.moveUp); err != nil {
			return err
		}
		if err := ui.gui.SetKeybinding(view, gocui.KeyArrowDown, gocui.ModNone, ui.moveDown); err != nil {
			return err
		}
	}

	// Update GRUB configuration
	if err := ui.gui.SetKeybinding("", 'g', gocui.ModNone, ui.updateGrub); err != nil {
		return err
	}

	// Execute btrfs balance
	if err := ui.gui.SetKeybinding("", 'b', gocui.ModNone, ui.executeBtrfsBalance); err != nil {
		return err
	}

	return nil
}

// executeBtrfsBalance executes btrfs balance
func (ui *UI) executeBtrfsBalance(g *gocui.Gui, v *gocui.View) error {
	message := "Are you sure you want to execute btrfs balance?\nThis operation may take a long time."
	return ui.showConfirmationDialog(message, func(g *gocui.Gui, v *gocui.View) error {
		output, err := ExecuteBtrfsBalance(defaultBtrfsPath)
		if err != nil {
			return ui.showDialog(fmt.Sprintf("Error executing btrfs balance:\n%v", err))
		}
		return ui.showDialog(fmt.Sprintf("Btrfs balance completed:\n%s", output))
	})
}

// isDialogVisible checks if a dialog window is currently displayed
func (ui *UI) isDialogVisible() bool {
	_, err := ui.gui.View(viewDialog)
	return err == nil
}

func (ui *UI) nextView(g *gocui.Gui, v *gocui.View) error {
	if ui.isDialogVisible() {
		return nil
	}
	views := []string{viewSubvolumes, viewSnapshots}
	for i, view := range views {
		if view == ui.currentView {
			if i < len(views)-1 {
				ui.currentView = views[i+1]
			} else {
				ui.currentView = views[0]
			}
			break
		}
	}
	if _, err := g.SetCurrentView(ui.currentView); err != nil {
		return err
	}

	// Update highlight for current view
	viewsList := []string{viewSubvolumes, viewSnapshots}
	for _, viewName := range viewsList {
		v, err := g.View(viewName)
		if err != nil {
			continue
		}
		v.Highlight = (viewName == ui.currentView)
	}

	// If switched to snapshots, update information
	if ui.currentView == viewSnapshots {
		ui.UpdateViewContent()
	} else {
		ui.updateSnapshotInfo()
	}

	// Update hotkeys display
	ui.updateHotkeys()
	return nil
}

func (ui *UI) prevView(g *gocui.Gui, v *gocui.View) error {
	if ui.isDialogVisible() {
		return nil
	}
	views := []string{viewSubvolumes, viewSnapshots}
	for i, view := range views {
		if view == ui.currentView {
			if i > 0 {
				ui.currentView = views[i-1]
			} else {
				ui.currentView = views[len(views)-1]
			}
			break
		}
	}
	if _, err := g.SetCurrentView(ui.currentView); err != nil {
		return err
	}

	// Update highlight for current view
	viewsList := []string{viewSubvolumes, viewSnapshots}
	for _, viewName := range viewsList {
		v, err := g.View(viewName)
		if err != nil {
			continue
		}
		v.Highlight = (viewName == ui.currentView)
	}

	// If switched to snapshots, update information
	if ui.currentView == viewSnapshots {
		ui.UpdateViewContent()
	} else {
		ui.updateSnapshotInfo()
	}

	// Update hotkeys display
	ui.updateHotkeys()
	return nil
}

func (ui *UI) moveUp(g *gocui.Gui, v *gocui.View) error {
	if v == nil || ui.isDialogVisible() {
		return nil
	}

	var data *ViewData
	if v.Name() == viewSubvolumes {
		data = ui.subvolumesData
	} else if v.Name() == viewSnapshots {
		data = ui.snapshotsData
	} else {
		return nil
	}

	data.MoveUp()
	data.Render(v)

	if v.Name() == viewSubvolumes {
		ui.UpdateViewContent()
	} else {
		ui.updateSnapshotInfo()
	}
	return nil
}

func (ui *UI) moveDown(g *gocui.Gui, v *gocui.View) error {
	if v == nil || ui.isDialogVisible() {
		return nil
	}

	var data *ViewData
	if v.Name() == viewSubvolumes {
		data = ui.subvolumesData
	} else if v.Name() == viewSnapshots {
		data = ui.snapshotsData
	} else {
		return nil
	}

	data.MoveDown()
	data.Render(v)

	if v.Name() == viewSubvolumes {
		ui.UpdateViewContent()
	} else {
		ui.updateSnapshotInfo()
	}
	return nil
}

func (ui *UI) deleteSnapshot(g *gocui.Gui, v *gocui.View) error {
	if len(ui.snapshotsData.items) == 0 || ui.isDialogVisible() {
		return nil
	}

	selectedSnapshot := ui.snapshotsData.GetSelected()
	if selectedSnapshot == "" {
		return nil
	}

	// Create confirmation message
	message := fmt.Sprintf("Are you sure you want to delete snapshot:\n%s?", selectedSnapshot)

	// Show confirmation dialog
	return ui.showConfirmationDialog(message, func(g *gocui.Gui, v *gocui.View) error {
		// Create full path to snapshot
		fullPath := fmt.Sprintf("%s/%s", defaultBtrfsPath, selectedSnapshot)

		// Delete snapshot
		if err := DeleteSnapshot(fullPath); err != nil {
			return err
		}

		// Update display
		ui.UpdateViewContent()
		return nil
	})
}

func (ui *UI) createSnapshot(g *gocui.Gui, v *gocui.View) error {
	if len(ui.subvolumesData.items) == 0 || ui.isDialogVisible() {
		return nil
	}

	selectedSubvol := ui.subvolumesData.GetSelected()
	if selectedSubvol == "" {
		return nil
	}

	// Extract base subvolume name
	parts := strings.Split(selectedSubvol, "/")
	if len(parts) < 2 {
		return nil
	}
	subvolBase := strings.Split(parts[1], "-")[0]

	// Create confirmation message
	message := fmt.Sprintf("Are you sure you want to create snapshot for:\n%s?", selectedSubvol)

	// Show confirmation dialog
	return ui.showConfirmationDialog(message, func(g *gocui.Gui, v *gocui.View) error {
		// Create path for new snapshot
		timestamp := time.Now().Format("20060102-150405")
		snapshotName := fmt.Sprintf("%s/%s-%s", snapshotPrefix, subvolBase, timestamp)
		sourcePath := fmt.Sprintf("%s/%s", defaultBtrfsPath, selectedSubvol)
		destPath := fmt.Sprintf("%s/%s", defaultBtrfsPath, snapshotName)

		// Create snapshot
		if err := CreateSnapshot(sourcePath, destPath); err != nil {
			return err
		}

		// Update display
		ui.UpdateViewContent()
		return nil
	})
}

// updateHotkeys updates hotkey display based on current view
func (ui *UI) updateHotkeys() {
	hotkeyView, err := ui.gui.View(viewHotkeys)
	if err != nil {
		return
	}
	hotkeyView.Clear()

	if ui.isDialogVisible() {
		dialogView, err := ui.gui.View(viewDialog)
		if err != nil {
			return
		}
		
		// Check dialog type by title
		if dialogView.Title == "GRUB Update" {
			fmt.Fprint(hotkeyView, "Enter: Close")
		} else {
			fmt.Fprint(hotkeyView, "Enter: Execute | c: Cancel")
		}
		return
	}

	baseHotkeys := "q: Quit | ←/→: Switch view | ↑/↓: Navigate | g: Update GRUB | b: Btrfs balance"
	if ui.currentView == viewSubvolumes {
		fmt.Fprintf(hotkeyView, "%s | t: Create snapshot", baseHotkeys)
	} else if ui.currentView == viewSnapshots {
		fmt.Fprintf(hotkeyView, "%s | r: Remove snapshot", baseHotkeys)
	} else {
		fmt.Fprint(hotkeyView, baseHotkeys)
	}
}

// showDialog displays a dialog window with a message
func (ui *UI) showDialog(message string) error {
	maxX, maxY := ui.gui.Size()
	width := 60
	height := 10
	x := maxX/2 - width/2
	y := maxY/2 - height/2

	v, err := ui.gui.SetView(viewDialog, x, y, x+width, y+height)
	if err != nil && err != gocui.ErrUnknownView {
		return err
	}

	v.Title = "GRUB Update"
	v.Wrap = true
	v.Clear()
	fmt.Fprintln(v, message)
	
	// Add closing instruction
	fmt.Fprintln(v, "\nPress Enter to close")

	if _, err := ui.gui.SetCurrentView(viewDialog); err != nil {
		return err
	}

	// Add handler for dialog closing
	if err := ui.gui.SetKeybinding(viewDialog, gocui.KeyEnter, gocui.ModNone,
		func(g *gocui.Gui, v *gocui.View) error {
			return ui.closeDialog()
		}); err != nil {
		return err
	}

	ui.updateHotkeys()
	return nil
}

// showConfirmationDialog displays an action confirmation dialog
func (ui *UI) showConfirmationDialog(message string, confirmAction func(*gocui.Gui, *gocui.View) error) error {
	maxX, maxY := ui.gui.Size()
	width := 60
	height := 10
	x := maxX/2 - width/2
	y := maxY/2 - height/2

	v, err := ui.gui.SetView(viewDialog, x, y, x+width, y+height)
	if err != nil && err != gocui.ErrUnknownView {
		return err
	}

	v.Title = "Action Confirmation"
	v.Wrap = true
	v.Clear()
	fmt.Fprintln(v, message)

	if _, err := ui.gui.SetCurrentView(viewDialog); err != nil {
		return err
	}

	// Confirmation handler
	if err := ui.gui.SetKeybinding(viewDialog, gocui.KeyEnter, gocui.ModNone,
		func(g *gocui.Gui, v *gocui.View) error {
			ui.closeDialog()
			return confirmAction(g, v)
		}); err != nil {
		return err
	}

	// Cancel handler
	if err := ui.gui.SetKeybinding(viewDialog, 'c', gocui.ModNone,
		func(g *gocui.Gui, v *gocui.View) error {
			return ui.closeDialog()
		}); err != nil {
		return err
	}

	ui.updateHotkeys()
	return nil
}

// closeDialog closes the dialog window
func (ui *UI) closeDialog() error {
	// Check if dialog window exists before deletion
	if v, _ := ui.gui.View(viewDialog); v != nil {
		// Remove Enter key handler
		ui.gui.DeleteKeybinding(viewDialog, gocui.KeyEnter, gocui.ModNone)
		
		if err := ui.gui.DeleteView(viewDialog); err != nil && err != gocui.ErrUnknownView {
			return err
		}
	}
	
	if _, err := ui.gui.SetCurrentView(ui.currentView); err != nil {
		return err
	}
	ui.updateHotkeys()
	return nil
}

// updateGrub updates GRUB configuration
func (ui *UI) updateGrub(g *gocui.Gui, v *gocui.View) error {
	output, err := ExecuteCommand("grub-mkconfig", "-o", "/boot/grub/grub.cfg")
	if err != nil {
		return ui.showDialog(fmt.Sprintf("Error updating GRUB:\n%v", err))
	}
	return ui.showDialog(fmt.Sprintf("GRUB successfully updated:\n%s", output))
}

func quit(g *gocui.Gui, v *gocui.View) error {
	return gocui.ErrQuit
}
