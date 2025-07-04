package driver

import (
	"context"
	"testing"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/stretchr/testify/assert"
	"k8s.io/mount-utils"
	"k8s.io/utils/exec"
	"k8s.io/utils/exec/testing"
)

const (
	stagingTargetPath = "/mnt" // Example staging target path
)

func getTestNodeServer() *NodeServer {
	driver := &Driver{
		name:               DefaultDriverName,
		version:            driverVersion,
		grpcServerEndpoint: DefaultGRPCServerEndpoint,
		nodeID:             "test-node-id",
	}
	commandScriptArray := []testingexec.FakeCommandAction{}
	//TODO: Simulate real commands
	for i := 0; i < 10; i++ {
		commandScriptArray = append(commandScriptArray, func(cmd string, args ...string) exec.Cmd {
			return &testingexec.FakeCmd{
				Argv:           append([]string{cmd}, args...),
				Stdout:         nil,
				Stderr:         nil,
				DisableScripts: true, // Disable script checking for simplicity
			}
		})
	}
	mounter := mount.NewSafeFormatAndMount(
		mount.NewFakeMounter([]mount.MountPoint{
			mount.MountPoint{
				Path: stagingTargetPath,
			},
		}), // using fake mounter implementation
		&testingexec.FakeExec{
			CommandScript: commandScriptArray,
		}, // using fake exec implementation
	)
	return NewNodeServer(driver, mounter)
}

func TestStageVolume(t *testing.T) {

	tcs := []struct {
		name                   string
		nodeStageVolumeRequest *csi.NodeStageVolumeRequest
		expectResponse         *csi.NodeStageVolumeResponse
		expectError            bool
	}{
		{
			name: "TestBasicMount",
			nodeStageVolumeRequest: &csi.NodeStageVolumeRequest{
				VolumeId:          "test-volume-id",
				StagingTargetPath: stagingTargetPath,
				VolumeCapability: &csi.VolumeCapability{
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{
							FsType: "ext4",
						},
					},
				},
				VolumeContext: map[string]string{
					"volumeName": "zero",
				},
			},
			expectResponse: &csi.NodeStageVolumeResponse{},
			expectError:    false,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			ns := getTestNodeServer()
			response, err := ns.NodeStageVolume(context.Background(), tc.nodeStageVolumeRequest)
			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			if tc.expectResponse != nil {
				assert.Equal(t, tc.expectResponse, response)
			} else {
				assert.NotNil(t, response)
			}
		})
	}
}
