package ui

import (
	"fmt"
	"strings"

	"github.com/jroimartin/gocui"
)

// ViewData stores data for display in the view
type ViewData struct {
	items    []string
	selected int
}

// NewViewData creates a new instance of ViewData
func NewViewData() *ViewData {
	return &ViewData{
		items:    make([]string, 0),
		selected: 0,
	}
}

// SetItems sets the list of items to display
func (vd *ViewData) SetItems(items []string) {
	vd.items = items
	if vd.selected >= len(items) {
		vd.selected = len(items) - 1
	}
	if vd.selected < 0 {
		vd.selected = 0
	}
}

// MoveUp moves the cursor up in the list
func (vd *ViewData) MoveUp() {
	if vd.selected > 0 {
		vd.selected--
	}
}

// MoveDown moves the cursor down in the list
func (vd *ViewData) MoveDown() {
	if vd.selected < len(vd.items)-1 {
		vd.selected++
	}
}

// GetSelected returns the selected item
func (vd *ViewData) GetSelected() string {
	if len(vd.items) == 0 {
		return ""
	}
	return vd.items[vd.selected]
}

// Render displays content in the view
func (vd *ViewData) Render(v *gocui.View) {
	v.Clear()
	
	_, viewHeight := v.Size()
	ox, oy := v.Origin()
	
	// Check if cursor is in visible area
	if vd.selected < oy {
		// Cursor is above visible area - scroll up
		v.SetOrigin(ox, vd.selected)
	} else if vd.selected >= oy+viewHeight {
		// Cursor is below visible area - scroll down
		v.SetOrigin(ox, vd.selected-viewHeight+1)
	}
	
	for i, item := range vd.items {
		if i == vd.selected {
			fmt.Fprintf(v, ">%s", item) // Use indentation for consistency
		} else {
			fmt.Fprintf(v, " %s", item)
		}
		if i < len(vd.items)-1 {
			fmt.Fprintln(v) // Add line break between items
		}
	}
	
	// Set cursor on selected line for highlighting
	v.SetCursor(0, vd.selected)
}

// updateSnapshotInfo updates information about selected snapshot
func (ui *UI) updateSnapshotInfo() {
	// Clear information if snapshots list is not selected
	if ui.currentView != viewSnapshots {
		infoView, err := ui.gui.View(viewSnapshotInfo)
		if err != nil {
			return
		}
		infoView.Clear()
		return
	}

	infoView, err := ui.gui.View(viewSnapshotInfo)
	if err != nil {
		return
	}
	infoView.Clear()

	// Get list of snapshots
	_, snapshots, err := GetBtrfsSubvolumes(defaultBtrfsPath)
	if err != nil {
		fmt.Fprintf(infoView, "Error getting snapshots: %v", err)
		return
	}

	if len(snapshots) == 0 {
		return
	}

	// Get selected snapshot from stored data
	if len(ui.snapshotsData.items) > 0 {
		selectedSnapshot := ui.snapshotsData.items[ui.snapshotsData.selected]
		fullPath := fmt.Sprintf("%s/%s", defaultBtrfsPath, selectedSnapshot)
		
		info, err := GetBtrfsSnapshotInfo(fullPath)
		if err != nil {
			fmt.Fprintf(infoView, "Error getting snapshot info: %v", err)
		} else {
			fmt.Fprintf(infoView, "Snapshot information:\n%s", info)
		}
	}
}

// UpdateViewContent updates view content with data from btrfs
func (ui *UI) UpdateViewContent() {
	subvolumes, snapshots, err := GetBtrfsSubvolumes(defaultBtrfsPath)
	if err != nil {
		// In case of error, show it in the view
		subvolView, _ := ui.gui.View(viewSubvolumes)
		if subvolView != nil {
			subvolView.Clear()
			fmt.Fprintf(subvolView, "Error: %v", err)
		}
		return
	}

	// Update subvolumes data
	ui.subvolumesData.SetItems(subvolumes)
	if subvolView, err := ui.gui.View(viewSubvolumes); err == nil {
		ui.subvolumesData.Render(subvolView)
	}

	// Update snapshots data
	snapView, err := ui.gui.View(viewSnapshots)
	if err == nil {
		selectedSubvol := ui.subvolumesData.selected
		if selectedSubvol >= 0 && selectedSubvol < len(subvolumes) {
			// Extract subvolume name (part after "/" and before "-")
			subvolName := subvolumes[selectedSubvol]
			parts := strings.Split(subvolName, "/")
			if len(parts) > 1 {
				subvolBase := strings.Split(parts[1], "-")[0]
				
				// Filter snapshots for selected subvolume
				filteredSnapshots := make([]string, 0)
				for _, snap := range snapshots {
					snapParts := strings.Split(snap, "/")
					if len(snapParts) > 1 {
						snapBase := strings.Split(snapParts[1], "-")[0]
						if snapBase == subvolBase {
							filteredSnapshots = append(filteredSnapshots, snap)
						}
					}
				}
				
				// Save current cursor position
				currentSelection := ui.snapshotsData.selected
				
				// Update snapshots data
				ui.snapshotsData.SetItems(filteredSnapshots)
				
				// Restore cursor position if valid
				if currentSelection >= 0 && currentSelection < len(filteredSnapshots) {
					ui.snapshotsData.selected = currentSelection
				}
				
				ui.snapshotsData.Render(snapView)
			}
		}
	}
	diskView, err := ui.gui.View(viewDiskInfo)
	if err == nil {
		diskInfo, err := GetDiskInfo(defaultBtrfsPath)
		if err != nil {
			fmt.Fprintf(diskView, "Error getting disk info: %v", err)
		} else {
			fmt.Fprintln(diskView, diskInfo)
		}
	}

	// Update selected snapshot information
	ui.updateSnapshotInfo()
}
