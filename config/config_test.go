package config

import (
	"os"
	"testing"

	"github.com/intxff/rdcross/log"
	"github.com/intxff/rdcross/util"
	"gopkg.in/yaml.v3"
)

func TestUnmarshal(t *testing.T) {
    t.Helper()
	config := RdConfig{}
	path, err := util.GetAbsPath("../config.yaml")
	if err != nil {
		log.Panic("invalid path")
	}

	buf, _ := os.ReadFile(path)

	err = yaml.Unmarshal(buf, &config)
	if err != nil {
        t.Fatalf("%v\n", err)
	}
}
