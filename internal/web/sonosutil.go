package web

import (
	"context"
	"log/slog"

	"audiobookshelf-sonos-bridge/internal/sonos"
)

// coordinatorIP returns the IP address of the group coordinator for the given
// device. If the device is standalone or an error occurs, the original device
// IP is returned. All AVTransport commands must go to the coordinator, which
// controls the entire group.
func coordinatorIP(ctx context.Context, deviceIP string) string {
	zgt := sonos.NewZoneGroupTopology(deviceIP)
	info, err := zgt.GetCoordinatorInfo(ctx)
	if err != nil {
		slog.Debug("could not get coordinator info, using device IP",
			"device_ip", deviceIP,
			"error", err)
		return deviceIP
	}
	if info.CoordinatorIP != "" && info.CoordinatorIP != deviceIP {
		slog.Debug("routing to group coordinator",
			"device_ip", deviceIP,
			"coordinator_ip", info.CoordinatorIP,
			"group_size", info.GroupSize)
		return info.CoordinatorIP
	}
	return deviceIP
}
