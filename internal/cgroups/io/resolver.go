package io

import (
	"context"
	"os"
	"path"

	"github.com/KonishchevDmitry/server-metrics/internal/logging"
)

type deviceResolver struct {
	devices map[string]string
}

func newDeviceResolver() *deviceResolver {
	return &deviceResolver{devices: make(map[string]string)}
}

func (r *deviceResolver) reset() {
	r.devices = make(map[string]string)
}

func (r *deviceResolver) getDeviceName(ctx context.Context, device string) string {
	if name, ok := r.devices[device]; ok {
		return name
	}

	name, err := os.Readlink(path.Join("/dev/char", device))
	if err != nil && os.IsNotExist(err) {
		name, err = os.Readlink(path.Join("/dev/block", device))
	}
	if err == nil {
		name = path.Base(name)
	} else {
		logging.L(ctx).Errorf("Failed to resolve %q device: %s.", device, err)
		name = device
	}

	r.devices[device] = name
	return name
}
