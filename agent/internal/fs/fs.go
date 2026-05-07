package fs

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/wirepanel/wirepanel/shared/proto"
)

const maxReadSize = 8 << 20

func List(path string) (json.RawMessage, error) {
	if path == "" {
		path = "/"
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	dirEntries, err := os.ReadDir(abs)
	if err != nil {
		return nil, err
	}
	out := proto.FSListResult{Path: abs, Entries: make([]proto.FSEntry, 0, len(dirEntries))}
	for _, de := range dirEntries {
		info, err := de.Info()
		if err != nil {
			continue
		}
		out.Entries = append(out.Entries, fsEntryFrom(info, filepath.Join(abs, info.Name())))
	}
	return json.Marshal(out)
}

func Stat(path string) (json.RawMessage, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, err
	}
	return json.Marshal(fsEntryFrom(info, abs))
}

func Read(path string) (json.RawMessage, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return nil, errors.New("path is directory")
	}
	if info.Size() > maxReadSize {
		return nil, errors.New("file too large")
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return nil, err
	}
	return json.Marshal(proto.FSReadResult{Path: abs, Content: string(data), Size: info.Size()})
}

func Write(path, content string, mode fs.FileMode) (json.RawMessage, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	if mode == 0 {
		mode = 0644
	}
	if err := os.WriteFile(abs, []byte(content), mode); err != nil {
		return nil, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, err
	}
	return json.Marshal(fsEntryFrom(info, abs))
}

func Delete(path string, recursive bool, trashID, trashDir string) (json.RawMessage, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	if abs == "/" {
		return nil, errors.New("refusing to delete /")
	}
	if trashID != "" && trashDir != "" {
		if err := os.MkdirAll(trashDir, 0o700); err != nil {
			return nil, err
		}
		dest := filepath.Join(trashDir, trashID)
		if err := os.Rename(abs, dest); err != nil {
			return nil, err
		}
		return json.Marshal(map[string]any{
			"deleted":      abs,
			"trashed_to":   dest,
			"recoverable":  true,
		})
	}
	if recursive {
		err = os.RemoveAll(abs)
	} else {
		err = os.Remove(abs)
	}
	if err != nil {
		return nil, err
	}
	return json.Marshal(map[string]any{"deleted": abs, "recoverable": false})
}

func Restore(trashPath, originalPath string) (json.RawMessage, error) {
	if trashPath == "" || originalPath == "" {
		return nil, errors.New("trash and original required")
	}
	if _, err := os.Stat(trashPath); err != nil {
		return nil, err
	}
	if _, err := os.Stat(originalPath); err == nil {
		return nil, errors.New("original path exists; refuse to overwrite on restore")
	}
	if err := os.Rename(trashPath, originalPath); err != nil {
		return nil, err
	}
	info, err := os.Stat(originalPath)
	if err != nil {
		return nil, err
	}
	return json.Marshal(fsEntryFrom(info, originalPath))
}

func Mkdir(path string, mode fs.FileMode) (json.RawMessage, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	if mode == 0 {
		mode = 0755
	}
	existed := false
	if _, err := os.Stat(abs); err == nil {
		existed = true
	}
	if err := os.MkdirAll(abs, mode); err != nil {
		return nil, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, err
	}
	out := map[string]any{
		"path":      abs,
		"existed":   existed,
		"size":      info.Size(),
		"mode":      info.Mode().String(),
		"is_dir":    info.IsDir(),
		"mod_time":  info.ModTime().UTC().Format(time.RFC3339),
	}
	return json.Marshal(out)
}

func fsEntryFrom(info os.FileInfo, abs string) proto.FSEntry {
	return proto.FSEntry{
		Name:    info.Name(),
		Path:    abs,
		Size:    info.Size(),
		Mode:    info.Mode().String(),
		IsDir:   info.IsDir(),
		ModTime: info.ModTime().UTC().Format(time.RFC3339),
	}
}
