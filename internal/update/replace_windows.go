//go:build windows

package update

import "os"

func replaceExecutable(source, target string) error {
	old := target + ".replacing"
	_ = os.Remove(old)
	if err := os.Rename(target, old); err != nil {
		return err
	}
	if err := os.Rename(source, target); err != nil {
		_ = os.Rename(old, target)
		return err
	}
	return os.Remove(old)
}

func replaceFile(source, target string) error {
	if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
		return err
	}
	return os.Rename(source, target)
}
