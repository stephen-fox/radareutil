// +build !windows

package radareutil

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func fullyQualifiedBinaryPath(exePath string) (string, error) {
	if !filepath.IsAbs(exePath) && !strings.ContainsAny("/", exePath) {
		whichOutputRaw, err := exec.Command("which", exePath).CombinedOutput()
		if err != nil {
			return exePath, fmt.Errorf("failed to lookup radare binary - %s - output: '%s'",
				err.Error(), whichOutputRaw)
		}

		exePath = string(bytes.TrimSpace(whichOutputRaw))

		_, err = os.Stat(exePath)
		if err != nil {
			return exePath, err
		}
	}

	return exePath, nil
}
