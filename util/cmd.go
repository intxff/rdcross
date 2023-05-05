package util

import (
	"errors"
	"os/exec"
	"strings"
)

func ExecCmd(cmd string) ([]byte, error) {
    c := strings.Split(cmd, " ")
    if c == nil {
        return nil, errors.New("empty cmd")
    }
    command := exec.Command(c[0], c[1:]...)
    return command.Output()
}
