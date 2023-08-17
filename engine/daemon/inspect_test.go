package daemon // import "github.com/docker/docker/daemon"

import (
	"testing"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/container"
	"github.com/docker/docker/daemon/config"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestGetInspectData(t *testing.T) {
	c := &container.Container{
		ID:           "inspect-me",
		HostConfig:   &containertypes.HostConfig{},
		State:        container.NewState(),
		ExecCommands: container.NewExecStore(),
	}

	d := &Daemon{
		linkIndex:   newLinkIndex(),
		configStore: &config.Config{},
	}
	if d.UsesSnapshotter() {
		t.Skip("does not apply to containerd snapshotters, which don't have RWLayer set")
	}
	_, err := d.getInspectData(c)
	assert.Check(t, is.ErrorContains(err, "RWLayer of container inspect-me is unexpectedly nil"))

	c.Dead = true
	_, err = d.getInspectData(c)
	assert.Check(t, err)
}
