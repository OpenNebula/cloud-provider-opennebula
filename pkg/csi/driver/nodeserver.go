/*
Copyright 2025, OpenNebula Project, OpenNebula Systems.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package driver

import (
	"context"
	"os"
	"path"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/kubernetes-csi/csi-lib-utils/protosanitizer"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	mount "k8s.io/mount-utils"
)

const (
	defaultFSType   = "ext4"  // Default filesystem type for volumes
	defaultDiskPath = "/dev/" // Path to disk devices (probably we should include in volumecontext)
)

type NodeServer struct {
	Driver    *Driver
	fsMounter *mount.SafeFormatAndMount
	csi.UnimplementedNodeServer
}

func NewNodeServer(d *Driver, mounter *mount.SafeFormatAndMount) *NodeServer {
	return &NodeServer{
		Driver:    d,
		fsMounter: mounter,
	}
}

//Following functions are RPC implementations defined in
// - https://github.com/container-storage-interface/spec/blob/master/spec.md#rpc-interface
// - https://github.com/container-storage-interface/spec/blob/master/spec.md#node-service-rpc

// The NodeStageVolume method behaves differently depending on the access type of the volume.
// For block access type, it skips mounting and formatting, while for mount access type, it
// performs the necessary operations to prepare the volume for use, like formatting and mounting
// the volume at the staging target path.
func (ns *NodeServer) NodeStageVolume(_ context.Context, req *csi.NodeStageVolumeRequest) (*csi.NodeStageVolumeResponse, error) {

	log.Debug().
		Str("args", protosanitizer.StripSecrets(req).String()).
		Msg("NodeStageVolume called")

	volumeID := req.GetVolumeId()
	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "volume ID is required")
	}

	stagingTargetPath := req.GetStagingTargetPath()
	if len(stagingTargetPath) == 0 {
		return nil, status.Error(codes.InvalidArgument, "staging target path is required")
	}

	// volume capability defines the access type (block or mount) and access mode of the volume
	volumeCapability := req.GetVolumeCapability()
	if volumeCapability == nil {
		return nil, status.Error(codes.InvalidArgument, "volume capability is required")
	}

	accessType := volumeCapability.GetAccessType()
	if _, ok := accessType.(*csi.VolumeCapability_Block); ok {
		// Block access type -> skip mounting and formatting
		log.Info().Msg("Block access type detected, skipping formatting and mounting")
		return &csi.NodeStageVolumeResponse{}, nil
	}

	volumeContext := req.GetVolumeContext()
	volName := volumeContext["volumeName"]
	if len(volName) == 0 {
		return nil, status.Error(codes.InvalidArgument, "[volumeName] entry is required in volume context")
	}
	devicePath := path.Join(defaultDiskPath, volName)

	volMount := volumeCapability.GetMount()
	mountFlags := volMount.GetMountFlags()
	fsType := volMount.GetFsType()
	if fsType == "" {
		fsType = defaultFSType
	}

	// Check if volume with volumeID exists
	if _, err := os.Stat(devicePath); os.IsNotExist(err) {
		log.Error().
			Str("volumeID", volumeID).
			Str("devicePath", devicePath).
			Msg("Device path does not exist, cannot format and mount volume")
		return nil, status.Error(codes.NotFound, "device path does not exist")
	}

	//TODO: Make a volume capabilities check

	//TODO: Check if volume_id is already staged in stagingTargetPath and is identical
	// to the volumeCapability provided in the request, then return 0 OK response

	//TODO: Check if the volume with volumeID is already staged at stagingTargetPath
	// but is incompatible with the volumeCapability provided in the request,
	// then return 6 ALREADY_EXISTS error

	// TODO: If the volume capability are not supported by the volume
	// rreturn 9 FAILED_PRECONDITION error

	isNotMountPoint, err := ns.fsMounter.IsLikelyNotMountPoint(stagingTargetPath)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to check if staging target path is a mount point")
	}
	if isNotMountPoint {
		log.Error().
			Str("stagingTargetPath", stagingTargetPath).
			Msg("Staging target path is not a valid mount point, cannot format and mount volume")
		return nil, status.Error(codes.FailedPrecondition, "staging target path is not a valid mount point")
	}

	log.Info().
		Str("devicePath", devicePath).
		Str("stagingTargetPath", stagingTargetPath).
		Str("fsType", fsType).
		Strs("mountFlags", mountFlags).
		Msg("Formatting and mounting volume")

	err = ns.fsMounter.FormatAndMount(devicePath, stagingTargetPath, fsType, mountFlags)
	if err != nil {
		log.Error().
			Err(err).
			Str("source", devicePath).
			Str("stagingTargetPath", stagingTargetPath).
			Str("fsType", fsType).
			Strs("mountFlags", mountFlags).
			Msg("Failed to format and mount volume")
		return nil, status.Error(codes.Internal, "failed to format and mount volume")
	}

	log.Info().Msg("Volume staged successfully")

	return &csi.NodeStageVolumeResponse{}, nil
}

func (ns *NodeServer) NodeUnstageVolume(_ context.Context, req *csi.NodeUnstageVolumeRequest) (*csi.NodeUnstageVolumeResponse, error) {
	//TODO: Implement
	return &csi.NodeUnstageVolumeResponse{}, nil
}

func (ns *NodeServer) NodePublishVolume(_ context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	//TODO: Implement
	return &csi.NodePublishVolumeResponse{}, nil
}

func (ns *NodeServer) NodeUnpublishVolume(_ context.Context, req *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
	//TODO: Implement
	return &csi.NodeUnpublishVolumeResponse{}, nil
}

func (ns *NodeServer) NodeGetVolumeStats(_ context.Context, req *csi.NodeGetVolumeStatsRequest) (*csi.NodeGetVolumeStatsResponse, error) {
	//TODO: Implement
	return &csi.NodeGetVolumeStatsResponse{}, nil
}

func (ns *NodeServer) NodeExpandVolume(_ context.Context, req *csi.NodeExpandVolumeRequest) (*csi.NodeExpandVolumeResponse, error) {
	//TODO: Implement
	return &csi.NodeExpandVolumeResponse{}, nil
}

func (ns *NodeServer) NodeGetCapabilities(_ context.Context, req *csi.NodeGetCapabilitiesRequest) (*csi.NodeGetCapabilitiesResponse, error) {
	//TODO: Implement
	return &csi.NodeGetCapabilitiesResponse{}, nil
}

func (ns *NodeServer) NodeGetInfo(_ context.Context, req *csi.NodeGetInfoRequest) (*csi.NodeGetInfoResponse, error) {
	//TODO: Implement
	return &csi.NodeGetInfoResponse{}, nil
}
