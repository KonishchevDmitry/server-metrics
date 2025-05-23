package classifier

import (
	"context"
	"testing"

	"github.com/samber/mo"
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

	traverse := mo.None[[]string]()
	total := func(exclude ...string) mo.Option[[]string] {
		return mo.Some(exclude)
	}

	for _, testCase := range []struct {
		group          string
		service        string
		totalExcluding mo.Option[[]string]
	}{
		{"/", "kernel", traverse},
		{"/init.scope", "init", traverse},
		{"/sys-fs-fuse-connections.mount", "sys-fs-fuse-connections.mount", traverse},

		{"/system.slice", "", traverse},
		{"/system.slice/boot-efi.mount", "boot-efi.mount", traverse},
		{"/system.slice/docker-3413aa74fd2ff75f15b32438dce58a63b73bc04c4bd476ca7ab54c12da6a43d4.scope", "server-metrics", traverse},
		{"/system.slice/docker-89eae77df5fb5de73ccc3eff21cd7f1c72434fef6ade1328924315ebe7eeadd5.scope", "docker-containers", traverse},
		{"/system.slice/nginx.service", "nginx", traverse},
		{"/system.slice/snap.shadowsocks-rust.ssserver-daemon-b5bad6a9-8ff1-4730-9f03-83b9d5998ddd.scope", "ssserver-daemon", traverse},
		{"/system.slice/system.slice:docker:jvifp9a6b1lxa1kuw8bwfcovf", "docker-builder", traverse},
		{`/system.slice/system-openvpn\x2dserver.slice`, "", traverse},
		{`/system.slice/system-openvpn\x2dserver.slice/openvpn-server@proxy.service`, "openvpn-server@proxy", traverse},
		{"/system.slice/systemd-udevd.service", "systemd-udevd", total()},
		{"/system.slice/systemd-journald-dev-log.socket", "systemd-journald-dev-log.socket", traverse},
		{`/system.slice/system-dbus\x2d:1.4\x2dorg.fedoraproject.SetroubleshootPrivileged.slice`, "dbus:org.fedoraproject.SetroubleshootPrivileged", total()},

		{"/user.slice", "", traverse},
		{"/user.slice/user-1000.slice", "dmitry/sessions", total("user@1000.service")},
		{"/user.slice/user-1000.slice/user@1000.service", "dmitry", total("app.slice", "init.scope")},
		{"/user.slice/user-1000.slice/user@1000.service/init.scope", "dmitry/init", traverse},
		{"/user.slice/user-1000.slice/user@1000.service/app.slice", "", traverse},
		{"/user.slice/user-1000.slice/user@1000.service/app.slice/dbus.socket", "dmitry/dbus.socket", traverse},
		{"/user.slice/user-1000.slice/user@1000.service/app.slice/app-vm.slice", "", traverse},
		{"/user.slice/user-1000.slice/user@1000.service/app.slice/app-vm.slice/vm@linux.service", "dmitry/vm@linux", traverse},
		{"/user.slice/user-1000.slice/user@1000.service/app.slice/ssh-agent.service", "dmitry/ssh-agent", traverse},
		{"/user.slice/user-1000.slice/user@1000.service/app.slice/snap.go.go-345c278e-7032-498e-8348-5c092e5d3623.scope", "dmitry/go", traverse},
		{"/user.slice/user-1000.slice/user@1000.service/app.slice/snap.shadowsocks-rust.ssserver-6f2a6b45-86b0-43fc-944f-d367b51e6c2f.scope", "dmitry/ssserver", traverse},
		{"/user.slice/user-1000.slice/user@1000.service/session.slice", "", traverse},
		{"/user.slice/user-1000.slice/user@1000.service/session.slice/dbus.service", "dmitry/dbus", traverse},
	} {
		t.Run(testCase.group, func(t *testing.T) {
			classification, ok, err := classifier.ClassifySlice(ctx, testCase.group)
			require.NoError(t, err)
			require.Equal(t, testCase.service != "", ok)
			require.Equal(t, testCase.service, classification.Service)
			require.Equal(t, testCase.totalExcluding, classification.TotalExcluding)
		})
	}
}
