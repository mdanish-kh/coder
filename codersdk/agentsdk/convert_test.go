package agentsdk_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"tailscale.com/tailcfg"

	"github.com/coder/coder/v2/agent/proto"
	"github.com/coder/coder/v2/codersdk"
	"github.com/coder/coder/v2/codersdk/agentsdk"
	"github.com/coder/coder/v2/tailnet"
)

func TestManifest(t *testing.T) {
	t.Parallel()
	manifest := agentsdk.Manifest{
		AgentID:            uuid.New(),
		AgentName:          "test-agent",
		OwnerName:          "test-owner",
		WorkspaceID:        uuid.New(),
		WorkspaceName:      "test-workspace",
		GitAuthConfigs:     3,
		VSCodePortProxyURI: "http://proxy.example.com/stuff",
		Apps: []codersdk.WorkspaceApp{
			{
				ID:            uuid.New(),
				URL:           "http://app1.example.com",
				External:      true,
				Slug:          "app1",
				DisplayName:   "App 1",
				Command:       "app1 -d",
				Icon:          "app1.png",
				Subdomain:     true,
				SubdomainName: "app1.example.com",
				SharingLevel:  codersdk.WorkspaceAppSharingLevelAuthenticated,
				Healthcheck: codersdk.Healthcheck{
					URL:       "http://localhost:3030/healthz",
					Interval:  55555666,
					Threshold: 55555666,
				},
				Health: codersdk.WorkspaceAppHealthHealthy,
			},
			{
				ID:            uuid.New(),
				URL:           "http://app2.example.com",
				External:      false,
				Slug:          "app2",
				DisplayName:   "App 2",
				Command:       "app2 -d",
				Icon:          "app2.png",
				Subdomain:     false,
				SubdomainName: "app2.example.com",
				SharingLevel:  codersdk.WorkspaceAppSharingLevelPublic,
				Healthcheck: codersdk.Healthcheck{
					URL:       "http://localhost:3032/healthz",
					Interval:  22555666,
					Threshold: 22555666,
				},
				Health: codersdk.WorkspaceAppHealthInitializing,
			},
		},
		DERPMap: &tailcfg.DERPMap{
			HomeParams: &tailcfg.DERPHomeParams{RegionScore: map[int]float64{999: 0.025}},
			Regions: map[int]*tailcfg.DERPRegion{
				999: {
					EmbeddedRelay: true,
					RegionID:      999,
					RegionCode:    "default",
					RegionName:    "HOME",
					Avoid:         false,
					Nodes: []*tailcfg.DERPNode{
						{
							Name: "Home1",
						},
					},
				},
			},
		},
		DERPForceWebSockets:      true,
		EnvironmentVariables:     map[string]string{"FOO": "bar"},
		Directory:                "/home/coder",
		MOTDFile:                 "/etc/motd",
		DisableDirectConnections: true,
		Metadata: []codersdk.WorkspaceAgentMetadataDescription{
			{
				DisplayName: "CPU",
				Key:         "cpu",
				Script:      "getcpu",
				Interval:    44444422,
				Timeout:     44444411,
			},
			{
				DisplayName: "MEM",
				Key:         "mem",
				Script:      "getmem",
				Interval:    54444422,
				Timeout:     54444411,
			},
		},
		Scripts: []codersdk.WorkspaceAgentScript{
			{
				LogSourceID:      uuid.New(),
				LogPath:          "/var/log/script.log",
				Script:           "script",
				Cron:             "somecron",
				RunOnStart:       true,
				RunOnStop:        true,
				StartBlocksLogin: true,
				Timeout:          time.Second,
			},
			{
				LogSourceID:      uuid.New(),
				LogPath:          "/var/log/script2.log",
				Script:           "script2",
				Cron:             "somecron2",
				RunOnStart:       false,
				RunOnStop:        true,
				StartBlocksLogin: true,
				Timeout:          time.Second * 4,
			},
		},
	}
	p, err := agentsdk.ProtoFromManifest(manifest)
	require.NoError(t, err)
	back, err := agentsdk.ManifestFromProto(p)
	require.NoError(t, err)
	require.Equal(t, manifest.AgentID, back.AgentID)
	require.Equal(t, manifest.AgentName, back.AgentName)
	require.Equal(t, manifest.OwnerName, back.OwnerName)
	require.Equal(t, manifest.WorkspaceID, back.WorkspaceID)
	require.Equal(t, manifest.WorkspaceName, back.WorkspaceName)
	require.Equal(t, manifest.GitAuthConfigs, back.GitAuthConfigs)
	require.Equal(t, manifest.VSCodePortProxyURI, back.VSCodePortProxyURI)
	require.Equal(t, manifest.Apps, back.Apps)
	require.NotNil(t, back.DERPMap)
	require.True(t, tailnet.CompareDERPMaps(manifest.DERPMap, back.DERPMap))
	require.Equal(t, manifest.DERPForceWebSockets, back.DERPForceWebSockets)
	require.Equal(t, manifest.EnvironmentVariables, back.EnvironmentVariables)
	require.Equal(t, manifest.Directory, back.Directory)
	require.Equal(t, manifest.MOTDFile, back.MOTDFile)
	require.Equal(t, manifest.DisableDirectConnections, back.DisableDirectConnections)
	require.Equal(t, manifest.Metadata, back.Metadata)
	require.Equal(t, manifest.Scripts, back.Scripts)
}

func TestSubsystems(t *testing.T) {
	t.Parallel()
	ss := []codersdk.AgentSubsystem{
		codersdk.AgentSubsystemEnvbox,
		codersdk.AgentSubsystemEnvbuilder,
		codersdk.AgentSubsystemExectrace,
	}
	ps, err := agentsdk.ProtoFromSubsystems(ss)
	require.NoError(t, err)
	require.Equal(t, ps, []proto.Startup_Subsystem{
		proto.Startup_ENVBOX,
		proto.Startup_ENVBUILDER,
		proto.Startup_EXECTRACE,
	})
}
