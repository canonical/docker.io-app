package daemon // import "github.com/docker/docker/integration/daemon"

import (
	"context"
	"runtime"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/testutil/daemon"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/skip"
)

func TestLiveRestore(t *testing.T) {
	skip.If(t, runtime.GOOS == "windows", "cannot start multiple daemons on windows")

	t.Run("volume references", testLiveRestoreVolumeReferences)
}

func testLiveRestoreVolumeReferences(t *testing.T) {
	t.Parallel()

	d := daemon.New(t)
	d.StartWithBusybox(t, "--live-restore", "--iptables=false")
	defer func() {
		d.Stop(t)
		d.Cleanup(t)
	}()

	c := d.NewClientT(t)
	ctx := context.Background()

	runTest := func(t *testing.T, policy string) {
		t.Run(policy, func(t *testing.T) {
			volName := "test-live-restore-volume-references-" + policy
			_, err := c.VolumeCreate(ctx, volume.VolumeCreateBody{Name: volName})
			assert.NilError(t, err)

			// Create a container that uses the volume
			m := mount.Mount{
				Type:   mount.TypeVolume,
				Source: volName,
				Target: "/foo",
			}
			cID := container.Run(ctx, t, c, container.WithMount(m), container.WithCmd("top"), container.WithRestartPolicy(policy))
			defer c.ContainerRemove(ctx, cID, types.ContainerRemoveOptions{Force: true})

			// Stop the daemon
			d.Restart(t, "--live-restore", "--iptables=false")

			// Try to remove the volume
			err = c.VolumeRemove(ctx, volName, false)
			assert.ErrorContains(t, err, "volume is in use")

			_, err = c.VolumeInspect(ctx, volName)
			assert.NilError(t, err)
		})
	}

	t.Run("restartPolicy", func(t *testing.T) {
		runTest(t, "always")
		runTest(t, "unless-stopped")
		runTest(t, "on-failure")
		runTest(t, "no")
	})
}
