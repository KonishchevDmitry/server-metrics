package classifier

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KonishchevDmitry/server-metrics/internal/docker"
	"github.com/KonishchevDmitry/server-metrics/internal/users"
)

func TestClassifier(t *testing.T) {
	ctx := context.Background()

	userResolver := users.NewResolverMock(map[int]string{
		1000: "dmitry",
	})

	dockerResolver := docker.NewResolverMock(map[string]docker.Container{
		"3413aa74fd2ff75f15b32438dce58a63b73bc04c4bd476ca7ab54c12da6a43d4": {Name: "server-metrics"},
		"89eae77df5fb5de73ccc3eff21cd7f1c72434fef6ade1328924315ebe7eeadd5": {Temporary: true},
	})
	defer func() {
		require.NoError(t, dockerResolver.Close())
	}()

	classifier := New(userResolver, dockerResolver)

	for _, testCase := range []struct {
		group   string
		service string
		total   bool
	}{
		{"/", "kernel", false},
		{"/init.scope", "init", false},
		{"/sys-fs-fuse-connections.mount", "sys-fs-fuse-connections.mount", false},

		{"/system.slice", "", false},
		{"/system.slice/boot-efi.mount", "boot-efi.mount", false},
		{"/system.slice/docker-3413aa74fd2ff75f15b32438dce58a63b73bc04c4bd476ca7ab54c12da6a43d4.scope", "server-metrics", false},
		{"/system.slice/docker-89eae77df5fb5de73ccc3eff21cd7f1c72434fef6ade1328924315ebe7eeadd5.scope", "docker-containers", false},
		{"/system.slice/nginx.service", "nginx", false},
		{"/system.slice/system.slice:docker:jvifp9a6b1lxa1kuw8bwfcovf", "docker-builder", false},
		{"/system.slice/system-openvpn\\x2dserver.slice", "openvpn-server", true},
		{"/system.slice/systemd-journald-dev-log.socket", "systemd-journald-dev-log.socket", false},

		{"/user.slice", "", false},
		{"/user.slice/user-1000.slice", "dmitry", true},
	} {
		testCase := testCase
		t.Run(testCase.group, func(t *testing.T) {
			service, total, ok, err := classifier.ClassifySlice(ctx, testCase.group)
			require.NoError(t, err)
			require.Equal(t, testCase.service != "", ok)
			require.Equal(t, testCase.total, total)
			require.Equal(t, testCase.service, service)
		})
	}
}
