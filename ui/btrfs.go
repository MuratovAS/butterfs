package ui

import (
	"fmt"
	"os/exec"
	"strings"
)

var (
	subvolumePrefix = "_active"
	snapshotPrefix  = "_snapshots"
)

// GetBtrfsSubvolumes executes 'btrfs subvolume list' command and returns filtered results
func GetBtrfsSubvolumes(path string) (subvolumes []string, snapshots []string, err error) {
	cmd := exec.Command("btrfs", "subvolume", "list", path)
	output, err := cmd.Output()
	if err != nil {
		return nil, nil, err
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		
		// Output format: ID gen parent top level path
		if strings.Contains(line, "path ") {
			// Extract path after "path " keyword
			pathIndex := strings.Index(line, "path ") + 5
			path := strings.TrimSpace(line[pathIndex:])
			
			// Filter paths by prefix
			if strings.HasPrefix(path, subvolumePrefix+"/") {
				subvolumes = append(subvolumes, path)
			} else if strings.HasPrefix(path, snapshotPrefix+"/") {
				snapshots = append(snapshots, path)
			}
		}
	}

	return subvolumes, snapshots, nil
}

// GetDiskInfo executes 'df' command and returns human-readable disk information
func GetDiskInfo(path string) (string, error) {
	cmd := exec.Command("df", "-h", path)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	// Skip header and get only disk information
	lines := strings.Split(string(output), "\n")
	if len(lines) < 2 {
		return "", nil
	}

	// Split line into fields, removing extra spaces
	fields := strings.Fields(lines[1])
	if len(fields) < 6 {
		return "", nil
	}

	// Format output with labels in compact form
	return fmt.Sprintf("FS: %s  Size: %s  Used: %s (%s)  Avail: %s",
		fields[0], fields[1], fields[2], fields[4], fields[3]), nil
}

// GetBtrfsSnapshotInfo executes 'btrfs subvolume show' command and returns snapshot information
func GetBtrfsSnapshotInfo(snapshotPath string) (string, error) {
	cmd := exec.Command("btrfs", "subvolume", "show", snapshotPath)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	
	return string(output), nil
}

// CreateSnapshot creates a new snapshot for the specified subvolume
func CreateSnapshot(subvolumePath string, snapshotPath string) error {
	cmd := exec.Command("btrfs", "subvolume", "snapshot", subvolumePath, snapshotPath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create snapshot: %v", err)
	}
	return nil
}

// DeleteSnapshot deletes the specified snapshot
func DeleteSnapshot(snapshotPath string) error {
	cmd := exec.Command("btrfs", "subvolume", "delete", snapshotPath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to delete snapshot: %v", err)
	}
	return nil
}

// ExecuteCommand runs an arbitrary command with given arguments and returns its output
func ExecuteCommand(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to execute command: %v", err)
	}
	return string(output), nil
}

// ExecuteBtrfsBalance executes btrfs balance command for the specified path
func ExecuteBtrfsBalance(path string) (string, error) {
	return ExecuteCommand("btrfs", "balance", "start", "-dusage=15", path)
}
