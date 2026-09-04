//go:build !windows

package update

import "os"

func replaceExecutable(source, target string) error {
	return os.Rename(source, target)
}

func replaceFile(source, target string) error {
	return os.Rename(source, target)
}
