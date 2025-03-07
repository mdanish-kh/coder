package terraform_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"

	tfjson "github.com/hashicorp/terraform-json"
	"github.com/stretchr/testify/require"
	protobuf "google.golang.org/protobuf/proto"

	"github.com/coder/coder/v2/cryptorand"
	"github.com/coder/coder/v2/provisioner/terraform"
	"github.com/coder/coder/v2/provisionersdk/proto"
)

func TestConvertResources(t *testing.T) {
	t.Parallel()
	// nolint:dogsled
	_, filename, _, _ := runtime.Caller(0)
	type testCase struct {
		resources        []*proto.Resource
		parameters       []*proto.RichParameter
		gitAuthProviders []string
	}

	// If a user doesn't specify 'display_apps' then they default
	// into all apps except VSCode Insiders.
	displayApps := proto.DisplayApps{
		Vscode:               true,
		VscodeInsiders:       false,
		WebTerminal:          true,
		PortForwardingHelper: true,
		SshHelper:            true,
	}

	// nolint:paralleltest
	for folderName, expected := range map[string]testCase{
		// When a resource depends on another, the shortest route
		// to a resource should always be chosen for the agent.
		"chaining-resources": {
			resources: []*proto.Resource{{
				Name: "a",
				Type: "null_resource",
			}, {
				Name: "b",
				Type: "null_resource",
				Agents: []*proto.Agent{{
					Name:                     "main",
					OperatingSystem:          "linux",
					Architecture:             "amd64",
					Auth:                     &proto.Agent_Token{},
					ConnectionTimeoutSeconds: 120,
					DisplayApps:              &displayApps,
				}},
			}},
		},
		// This can happen when resources hierarchically conflict.
		// When multiple resources exist at the same level, the first
		// listed in state will be chosen.
		"conflicting-resources": {
			resources: []*proto.Resource{{
				Name: "first",
				Type: "null_resource",
				Agents: []*proto.Agent{{
					Name:                     "main",
					OperatingSystem:          "linux",
					Architecture:             "amd64",
					Auth:                     &proto.Agent_Token{},
					ConnectionTimeoutSeconds: 120,
					DisplayApps:              &displayApps,
				}},
			}, {
				Name: "second",
				Type: "null_resource",
			}},
		},
		// Ensures the instance ID authentication type surfaces.
		"instance-id": {
			resources: []*proto.Resource{{
				Name: "main",
				Type: "null_resource",
				Agents: []*proto.Agent{{
					Name:                     "main",
					OperatingSystem:          "linux",
					Architecture:             "amd64",
					Auth:                     &proto.Agent_InstanceId{},
					ConnectionTimeoutSeconds: 120,
					DisplayApps:              &displayApps,
				}},
			}},
		},
		// Ensures that calls to resources through modules work
		// as expected.
		"calling-module": {
			resources: []*proto.Resource{{
				Name: "example",
				Type: "null_resource",
				Agents: []*proto.Agent{{
					Name:                     "main",
					OperatingSystem:          "linux",
					Architecture:             "amd64",
					Auth:                     &proto.Agent_Token{},
					ConnectionTimeoutSeconds: 120,
					DisplayApps:              &displayApps,
				}},
			}},
		},
		// Ensures the attachment of multiple agents to a single
		// resource is successful.
		"multiple-agents": {
			resources: []*proto.Resource{{
				Name: "dev",
				Type: "null_resource",
				Agents: []*proto.Agent{{
					Name:                     "dev1",
					OperatingSystem:          "linux",
					Architecture:             "amd64",
					Auth:                     &proto.Agent_Token{},
					ConnectionTimeoutSeconds: 120,
					DisplayApps:              &displayApps,
				}, {
					Name:                     "dev2",
					OperatingSystem:          "darwin",
					Architecture:             "amd64",
					Auth:                     &proto.Agent_Token{},
					ConnectionTimeoutSeconds: 1,
					MotdFile:                 "/etc/motd",
					DisplayApps:              &displayApps,
					Scripts: []*proto.Script{{
						Icon:        "/emojis/25c0.png",
						DisplayName: "Shutdown Script",
						RunOnStop:   true,
						LogPath:     "coder-shutdown-script.log",
						Script:      "echo bye bye",
					}},
				}, {
					Name:                     "dev3",
					OperatingSystem:          "windows",
					Architecture:             "arm64",
					Auth:                     &proto.Agent_Token{},
					ConnectionTimeoutSeconds: 120,
					TroubleshootingUrl:       "https://coder.com/troubleshoot",
					DisplayApps:              &displayApps,
				}, {
					Name:                     "dev4",
					OperatingSystem:          "linux",
					Architecture:             "amd64",
					Auth:                     &proto.Agent_Token{},
					ConnectionTimeoutSeconds: 120,
					DisplayApps:              &displayApps,
				}},
			}},
		},
		// Ensures multiple applications can be set for a single agent.
		"multiple-apps": {
			resources: []*proto.Resource{{
				Name: "dev",
				Type: "null_resource",
				Agents: []*proto.Agent{{
					Name:            "dev1",
					OperatingSystem: "linux",
					Architecture:    "amd64",
					Apps: []*proto.App{
						{
							Slug:        "app1",
							DisplayName: "app1",
							// Subdomain defaults to false if unspecified.
							Subdomain: false,
						},
						{
							Slug:        "app2",
							DisplayName: "app2",
							Subdomain:   true,
							Healthcheck: &proto.Healthcheck{
								Url:       "http://localhost:13337/healthz",
								Interval:  5,
								Threshold: 6,
							},
						},
						{
							Slug:        "app3",
							DisplayName: "app3",
							Subdomain:   false,
						},
					},
					Auth:                     &proto.Agent_Token{},
					ConnectionTimeoutSeconds: 120,
					DisplayApps:              &displayApps,
				}},
			}},
		},
		"mapped-apps": {
			resources: []*proto.Resource{{
				Name: "dev",
				Type: "null_resource",
				Agents: []*proto.Agent{{
					Name:            "dev",
					OperatingSystem: "linux",
					Architecture:    "amd64",
					Apps: []*proto.App{
						{
							Slug:        "app1",
							DisplayName: "app1",
						},
						{
							Slug:        "app2",
							DisplayName: "app2",
						},
					},
					Auth:                     &proto.Agent_Token{},
					ConnectionTimeoutSeconds: 120,
					DisplayApps:              &displayApps,
				}},
			}},
		},
		// Tests fetching metadata about workspace resources.
		"resource-metadata": {
			resources: []*proto.Resource{{
				Name:      "about",
				Type:      "null_resource",
				Hide:      true,
				Icon:      "/icon/server.svg",
				DailyCost: 29,
				Metadata: []*proto.Resource_Metadata{{
					Key:   "hello",
					Value: "world",
				}, {
					Key:    "null",
					IsNull: true,
				}, {
					Key: "empty",
				}, {
					Key:       "secret",
					Value:     "squirrel",
					Sensitive: true,
				}},
				Agents: []*proto.Agent{{
					Name:            "main",
					Auth:            &proto.Agent_Token{},
					OperatingSystem: "linux",
					Architecture:    "amd64",
					Metadata: []*proto.Agent_Metadata{{
						Key:         "process_count",
						DisplayName: "Process Count",
						Script:      "ps -ef | wc -l",
						Interval:    5,
						Timeout:     1,
						Order:       7,
					}},
					ConnectionTimeoutSeconds: 120,
					DisplayApps:              &displayApps,
				}},
			}},
		},
		// Tests that resources with the same id correctly get metadata applied
		// to them.
		"kubernetes-metadata": {
			resources: []*proto.Resource{
				{
					Name: "coder_workspace",
					Type: "kubernetes_config_map",
				}, {
					Name: "coder_workspace",
					Type: "kubernetes_role",
				}, {
					Name: "coder_workspace",
					Type: "kubernetes_role_binding",
				}, {
					Name: "coder_workspace",
					Type: "kubernetes_secret",
				}, {
					Name: "coder_workspace",
					Type: "kubernetes_service_account",
				}, {
					Name: "main",
					Type: "kubernetes_pod",
					Metadata: []*proto.Resource_Metadata{{
						Key:   "cpu",
						Value: "1",
					}, {
						Key:   "memory",
						Value: "1Gi",
					}, {
						Key:   "gpu",
						Value: "1",
					}},
					Agents: []*proto.Agent{{
						Name:            "main",
						OperatingSystem: "linux",
						Architecture:    "amd64",
						Apps: []*proto.App{
							{
								Icon:        "/icon/code.svg",
								Slug:        "code-server",
								DisplayName: "code-server",
								Url:         "http://localhost:13337?folder=/home/coder",
							},
						},
						Auth:                     &proto.Agent_Token{},
						ConnectionTimeoutSeconds: 120,
						DisplayApps:              &displayApps,
						Scripts: []*proto.Script{{
							DisplayName: "Startup Script",
							RunOnStart:  true,
							LogPath:     "coder-startup-script.log",
							Icon:        "/emojis/25b6.png",
							Script:      "    #!/bin/bash\n    # home folder can be empty, so copying default bash settings\n    if [ ! -f ~/.profile ]; then\n      cp /etc/skel/.profile $HOME\n    fi\n    if [ ! -f ~/.bashrc ]; then\n      cp /etc/skel/.bashrc $HOME\n    fi\n    # install and start code-server\n    curl -fsSL https://code-server.dev/install.sh | sh  | tee code-server-install.log\n    code-server --auth none --port 13337 | tee code-server-install.log &\n",
						}},
					}},
				},
			},
		},
		"rich-parameters": {
			resources: []*proto.Resource{{
				Name: "dev",
				Type: "null_resource",
				Agents: []*proto.Agent{{
					Name:                     "dev",
					OperatingSystem:          "windows",
					Architecture:             "arm64",
					Auth:                     &proto.Agent_Token{},
					ConnectionTimeoutSeconds: 120,
					DisplayApps:              &displayApps,
				}},
			}},
			parameters: []*proto.RichParameter{{
				Name:         "First parameter from child module",
				Type:         "string",
				Description:  "First parameter from child module",
				Mutable:      true,
				DefaultValue: "abcdef",
			}, {
				Name:         "Second parameter from child module",
				Type:         "string",
				Description:  "Second parameter from child module",
				Mutable:      true,
				DefaultValue: "ghijkl",
			}, {
				Name:         "First parameter from module",
				Type:         "string",
				Description:  "First parameter from module",
				Mutable:      true,
				DefaultValue: "abcdef",
			}, {
				Name:         "Second parameter from module",
				Type:         "string",
				Description:  "Second parameter from module",
				Mutable:      true,
				DefaultValue: "ghijkl",
			}, {
				Name: "Example",
				Type: "string",
				Options: []*proto.RichParameterOption{{
					Name:  "First Option",
					Value: "first",
				}, {
					Name:  "Second Option",
					Value: "second",
				}},
				Required: true,
			}, {
				Name:          "number_example",
				Type:          "number",
				DefaultValue:  "4",
				ValidationMin: nil,
				ValidationMax: nil,
			}, {
				Name:          "number_example_max_zero",
				Type:          "number",
				DefaultValue:  "-2",
				ValidationMin: terraform.PtrInt32(-3),
				ValidationMax: terraform.PtrInt32(0),
			}, {
				Name:          "number_example_min_max",
				Type:          "number",
				DefaultValue:  "4",
				ValidationMin: terraform.PtrInt32(3),
				ValidationMax: terraform.PtrInt32(6),
			}, {
				Name:          "number_example_min_zero",
				Type:          "number",
				DefaultValue:  "4",
				ValidationMin: terraform.PtrInt32(0),
				ValidationMax: terraform.PtrInt32(6),
			}, {
				Name:         "Sample",
				Type:         "string",
				Description:  "blah blah",
				DefaultValue: "ok",
			}},
		},
		"rich-parameters-order": {
			resources: []*proto.Resource{{
				Name: "dev",
				Type: "null_resource",
				Agents: []*proto.Agent{{
					Name:                     "dev",
					OperatingSystem:          "windows",
					Architecture:             "arm64",
					Auth:                     &proto.Agent_Token{},
					ConnectionTimeoutSeconds: 120,
					DisplayApps:              &displayApps,
				}},
			}},
			parameters: []*proto.RichParameter{{
				Name:     "Example",
				Type:     "string",
				Required: true,
				Order:    55,
			}, {
				Name:         "Sample",
				Type:         "string",
				Description:  "blah blah",
				DefaultValue: "ok",
				Order:        99,
			}},
		},
		"rich-parameters-validation": {
			resources: []*proto.Resource{{
				Name: "dev",
				Type: "null_resource",
				Agents: []*proto.Agent{{
					Name:                     "dev",
					OperatingSystem:          "windows",
					Architecture:             "arm64",
					Auth:                     &proto.Agent_Token{},
					ConnectionTimeoutSeconds: 120,
					DisplayApps:              &displayApps,
				}},
			}},
			parameters: []*proto.RichParameter{{
				Name:          "number_example",
				Type:          "number",
				DefaultValue:  "4",
				Ephemeral:     true,
				Mutable:       true,
				ValidationMin: nil,
				ValidationMax: nil,
			}, {
				Name:          "number_example_max",
				Type:          "number",
				DefaultValue:  "4",
				ValidationMin: nil,
				ValidationMax: terraform.PtrInt32(6),
			}, {
				Name:          "number_example_max_zero",
				Type:          "number",
				DefaultValue:  "-3",
				ValidationMin: nil,
				ValidationMax: terraform.PtrInt32(0),
			}, {
				Name:          "number_example_min",
				Type:          "number",
				DefaultValue:  "4",
				ValidationMin: terraform.PtrInt32(3),
				ValidationMax: nil,
			}, {
				Name:          "number_example_min_max",
				Type:          "number",
				DefaultValue:  "4",
				ValidationMin: terraform.PtrInt32(3),
				ValidationMax: terraform.PtrInt32(6),
			}, {
				Name:          "number_example_min_zero",
				Type:          "number",
				DefaultValue:  "4",
				ValidationMin: terraform.PtrInt32(0),
				ValidationMax: nil,
			}},
		},
		"git-auth-providers": {
			resources: []*proto.Resource{{
				Name: "dev",
				Type: "null_resource",
				Agents: []*proto.Agent{{
					Name:                     "main",
					OperatingSystem:          "linux",
					Architecture:             "amd64",
					Auth:                     &proto.Agent_Token{},
					ConnectionTimeoutSeconds: 120,
					DisplayApps:              &displayApps,
				}},
			}},
			gitAuthProviders: []string{"github", "gitlab"},
		},
		"display-apps": {
			resources: []*proto.Resource{{
				Name: "dev",
				Type: "null_resource",
				Agents: []*proto.Agent{{
					Name:                     "main",
					OperatingSystem:          "linux",
					Architecture:             "amd64",
					Auth:                     &proto.Agent_Token{},
					ConnectionTimeoutSeconds: 120,
					DisplayApps: &proto.DisplayApps{
						VscodeInsiders: true,
						WebTerminal:    true,
					},
				}},
			}},
		},
		"display-apps-disabled": {
			resources: []*proto.Resource{{
				Name: "dev",
				Type: "null_resource",
				Agents: []*proto.Agent{{
					Name:                     "main",
					OperatingSystem:          "linux",
					Architecture:             "amd64",
					Auth:                     &proto.Agent_Token{},
					ConnectionTimeoutSeconds: 120,
					DisplayApps:              &proto.DisplayApps{},
				}},
			}},
		},
	} {
		folderName := folderName
		expected := expected
		t.Run(folderName, func(t *testing.T) {
			t.Parallel()
			dir := filepath.Join(filepath.Dir(filename), "testdata", folderName)
			t.Run("Plan", func(t *testing.T) {
				t.Parallel()

				tfPlanRaw, err := os.ReadFile(filepath.Join(dir, folderName+".tfplan.json"))
				require.NoError(t, err)
				var tfPlan tfjson.Plan
				err = json.Unmarshal(tfPlanRaw, &tfPlan)
				require.NoError(t, err)
				tfPlanGraph, err := os.ReadFile(filepath.Join(dir, folderName+".tfplan.dot"))
				require.NoError(t, err)

				modules := []*tfjson.StateModule{tfPlan.PlannedValues.RootModule}
				if tfPlan.PriorState != nil {
					modules = append(modules, tfPlan.PriorState.Values.RootModule)
				} else {
					// Ensure that resources can be duplicated in the source state
					// and that no errors occur!
					modules = append(modules, tfPlan.PlannedValues.RootModule)
				}
				state, err := terraform.ConvertState(modules, string(tfPlanGraph))
				require.NoError(t, err)
				sortResources(state.Resources)
				sort.Strings(state.ExternalAuthProviders)

				expectedNoMetadata := make([]*proto.Resource, 0)
				for _, resource := range expected.resources {
					resourceCopy, _ := protobuf.Clone(resource).(*proto.Resource)
					// plan cannot know whether values are null or not
					for _, metadata := range resourceCopy.Metadata {
						metadata.IsNull = false
					}
					expectedNoMetadata = append(expectedNoMetadata, resourceCopy)
				}

				// Convert expectedNoMetadata and resources into a
				// []map[string]interface{} so they can be compared easily.
				data, err := json.Marshal(expectedNoMetadata)
				require.NoError(t, err)
				var expectedNoMetadataMap []map[string]interface{}
				err = json.Unmarshal(data, &expectedNoMetadataMap)
				require.NoError(t, err)

				data, err = json.Marshal(state.Resources)
				require.NoError(t, err)
				var resourcesMap []map[string]interface{}
				err = json.Unmarshal(data, &resourcesMap)
				require.NoError(t, err)
				require.Equal(t, expectedNoMetadataMap, resourcesMap)

				expectedParams := expected.parameters
				if expectedParams == nil {
					expectedParams = []*proto.RichParameter{}
				}
				parametersWant, err := json.Marshal(expectedParams)
				require.NoError(t, err)
				parametersGot, err := json.Marshal(state.Parameters)
				require.NoError(t, err)
				require.Equal(t, string(parametersWant), string(parametersGot))
				require.Equal(t, expectedNoMetadataMap, resourcesMap)

				require.ElementsMatch(t, expected.gitAuthProviders, state.ExternalAuthProviders)
			})

			t.Run("Provision", func(t *testing.T) {
				t.Parallel()
				tfStateRaw, err := os.ReadFile(filepath.Join(dir, folderName+".tfstate.json"))
				require.NoError(t, err)
				var tfState tfjson.State
				err = json.Unmarshal(tfStateRaw, &tfState)
				require.NoError(t, err)
				tfStateGraph, err := os.ReadFile(filepath.Join(dir, folderName+".tfstate.dot"))
				require.NoError(t, err)

				state, err := terraform.ConvertState([]*tfjson.StateModule{tfState.Values.RootModule}, string(tfStateGraph))
				require.NoError(t, err)
				sortResources(state.Resources)
				sort.Strings(state.ExternalAuthProviders)
				for _, resource := range state.Resources {
					for _, agent := range resource.Agents {
						agent.Id = ""
						if agent.GetToken() != "" {
							agent.Auth = &proto.Agent_Token{}
						}
						if agent.GetInstanceId() != "" {
							agent.Auth = &proto.Agent_InstanceId{}
						}
					}
				}
				// Convert expectedNoMetadata and resources into a
				// []map[string]interface{} so they can be compared easily.
				data, err := json.Marshal(expected.resources)
				require.NoError(t, err)
				var expectedMap []map[string]interface{}
				err = json.Unmarshal(data, &expectedMap)
				require.NoError(t, err)

				data, err = json.Marshal(state.Resources)
				require.NoError(t, err)
				var resourcesMap []map[string]interface{}
				err = json.Unmarshal(data, &resourcesMap)
				require.NoError(t, err)

				require.Equal(t, expectedMap, resourcesMap)
				require.ElementsMatch(t, expected.gitAuthProviders, state.ExternalAuthProviders)
			})
		})
	}
}

func TestAppSlugValidation(t *testing.T) {
	t.Parallel()

	// nolint:dogsled
	_, filename, _, _ := runtime.Caller(0)

	// Load the multiple-apps state file and edit it.
	dir := filepath.Join(filepath.Dir(filename), "testdata", "multiple-apps")
	tfPlanRaw, err := os.ReadFile(filepath.Join(dir, "multiple-apps.tfplan.json"))
	require.NoError(t, err)
	var tfPlan tfjson.Plan
	err = json.Unmarshal(tfPlanRaw, &tfPlan)
	require.NoError(t, err)
	tfPlanGraph, err := os.ReadFile(filepath.Join(dir, "multiple-apps.tfplan.dot"))
	require.NoError(t, err)

	// Change all slugs to be invalid.
	for _, resource := range tfPlan.PlannedValues.RootModule.Resources {
		if resource.Type == "coder_app" {
			resource.AttributeValues["slug"] = "$$$ invalid slug $$$"
		}
	}

	state, err := terraform.ConvertState([]*tfjson.StateModule{tfPlan.PlannedValues.RootModule}, string(tfPlanGraph))
	require.Nil(t, state)
	require.Error(t, err)
	require.ErrorContains(t, err, "invalid app slug")

	// Change all slugs to be identical and valid.
	for _, resource := range tfPlan.PlannedValues.RootModule.Resources {
		if resource.Type == "coder_app" {
			resource.AttributeValues["slug"] = "valid"
		}
	}

	state, err = terraform.ConvertState([]*tfjson.StateModule{tfPlan.PlannedValues.RootModule}, string(tfPlanGraph))
	require.Nil(t, state)
	require.Error(t, err)
	require.ErrorContains(t, err, "duplicate app slug")
}

func TestMetadataResourceDuplicate(t *testing.T) {
	t.Parallel()

	// Load the multiple-apps state file and edit it.
	dir := filepath.Join("testdata", "resource-metadata-duplicate")
	tfPlanRaw, err := os.ReadFile(filepath.Join(dir, "resource-metadata-duplicate.tfplan.json"))
	require.NoError(t, err)
	var tfPlan tfjson.Plan
	err = json.Unmarshal(tfPlanRaw, &tfPlan)
	require.NoError(t, err)
	tfPlanGraph, err := os.ReadFile(filepath.Join(dir, "resource-metadata-duplicate.tfplan.dot"))
	require.NoError(t, err)

	state, err := terraform.ConvertState([]*tfjson.StateModule{tfPlan.PlannedValues.RootModule}, string(tfPlanGraph))
	require.Nil(t, state)
	require.Error(t, err)
	require.ErrorContains(t, err, "duplicate metadata resource: null_resource.about")
}

func TestParameterValidation(t *testing.T) {
	t.Parallel()

	// nolint:dogsled
	_, filename, _, _ := runtime.Caller(0)

	// Load the rich-parameters state file and edit it.
	dir := filepath.Join(filepath.Dir(filename), "testdata", "rich-parameters")
	tfPlanRaw, err := os.ReadFile(filepath.Join(dir, "rich-parameters.tfplan.json"))
	require.NoError(t, err)
	var tfPlan tfjson.Plan
	err = json.Unmarshal(tfPlanRaw, &tfPlan)
	require.NoError(t, err)
	tfPlanGraph, err := os.ReadFile(filepath.Join(dir, "rich-parameters.tfplan.dot"))
	require.NoError(t, err)

	// Change all names to be identical.
	var names []string
	for _, resource := range tfPlan.PriorState.Values.RootModule.Resources {
		if resource.Type == "coder_parameter" {
			resource.AttributeValues["name"] = "identical"
			names = append(names, resource.Name)
		}
	}

	state, err := terraform.ConvertState([]*tfjson.StateModule{tfPlan.PriorState.Values.RootModule}, string(tfPlanGraph))
	require.Nil(t, state)
	require.Error(t, err)
	require.ErrorContains(t, err, "coder_parameter names must be unique but \"identical\" appears multiple times")

	// Make two sets of identical names.
	count := 0
	names = nil
	for _, resource := range tfPlan.PriorState.Values.RootModule.Resources {
		if resource.Type == "coder_parameter" {
			resource.AttributeValues["name"] = fmt.Sprintf("identical-%d", count%2)
			names = append(names, resource.Name)
			count++
		}
	}

	state, err = terraform.ConvertState([]*tfjson.StateModule{tfPlan.PriorState.Values.RootModule}, string(tfPlanGraph))
	require.Nil(t, state)
	require.Error(t, err)
	require.ErrorContains(t, err, "coder_parameter names must be unique but \"identical-0\" and \"identical-1\" appear multiple times")

	// Once more with three sets.
	count = 0
	names = nil
	for _, resource := range tfPlan.PriorState.Values.RootModule.Resources {
		if resource.Type == "coder_parameter" {
			resource.AttributeValues["name"] = fmt.Sprintf("identical-%d", count%3)
			names = append(names, resource.Name)
			count++
		}
	}

	state, err = terraform.ConvertState([]*tfjson.StateModule{tfPlan.PriorState.Values.RootModule}, string(tfPlanGraph))
	require.Nil(t, state)
	require.Error(t, err)
	require.ErrorContains(t, err, "coder_parameter names must be unique but \"identical-0\", \"identical-1\" and \"identical-2\" appear multiple times")
}

func TestInstanceTypeAssociation(t *testing.T) {
	t.Parallel()
	type tc struct {
		ResourceType    string
		InstanceTypeKey string
	}
	for _, tc := range []tc{{
		ResourceType:    "google_compute_instance",
		InstanceTypeKey: "machine_type",
	}, {
		ResourceType:    "aws_instance",
		InstanceTypeKey: "instance_type",
	}, {
		ResourceType:    "aws_spot_instance_request",
		InstanceTypeKey: "instance_type",
	}, {
		ResourceType:    "azurerm_linux_virtual_machine",
		InstanceTypeKey: "size",
	}, {
		ResourceType:    "azurerm_windows_virtual_machine",
		InstanceTypeKey: "size",
	}} {
		tc := tc
		t.Run(tc.ResourceType, func(t *testing.T) {
			t.Parallel()
			instanceType, err := cryptorand.String(12)
			require.NoError(t, err)
			state, err := terraform.ConvertState([]*tfjson.StateModule{{
				Resources: []*tfjson.StateResource{{
					Address: tc.ResourceType + ".dev",
					Type:    tc.ResourceType,
					Name:    "dev",
					Mode:    tfjson.ManagedResourceMode,
					AttributeValues: map[string]interface{}{
						tc.InstanceTypeKey: instanceType,
					},
				}},
				// This is manually created to join the edges.
			}}, `digraph {
	compound = "true"
	newrank = "true"
	subgraph "root" {
		"[root] `+tc.ResourceType+`.dev" [label = "`+tc.ResourceType+`.dev", shape = "box"]
	}
}`)
			require.NoError(t, err)
			require.Len(t, state.Resources, 1)
			require.Equal(t, state.Resources[0].GetInstanceType(), instanceType)
		})
	}
}

func TestInstanceIDAssociation(t *testing.T) {
	t.Parallel()
	type tc struct {
		Auth          string
		ResourceType  string
		InstanceIDKey string
	}
	for _, tc := range []tc{{
		Auth:          "google-instance-identity",
		ResourceType:  "google_compute_instance",
		InstanceIDKey: "instance_id",
	}, {
		Auth:          "aws-instance-identity",
		ResourceType:  "aws_instance",
		InstanceIDKey: "id",
	}, {
		Auth:          "aws-instance-identity",
		ResourceType:  "aws_spot_instance_request",
		InstanceIDKey: "spot_instance_id",
	}, {
		Auth:          "azure-instance-identity",
		ResourceType:  "azurerm_linux_virtual_machine",
		InstanceIDKey: "virtual_machine_id",
	}, {
		Auth:          "azure-instance-identity",
		ResourceType:  "azurerm_windows_virtual_machine",
		InstanceIDKey: "virtual_machine_id",
	}} {
		tc := tc
		t.Run(tc.ResourceType, func(t *testing.T) {
			t.Parallel()
			instanceID, err := cryptorand.String(12)
			require.NoError(t, err)
			state, err := terraform.ConvertState([]*tfjson.StateModule{{
				Resources: []*tfjson.StateResource{{
					Address: "coder_agent.dev",
					Type:    "coder_agent",
					Name:    "dev",
					Mode:    tfjson.ManagedResourceMode,
					AttributeValues: map[string]interface{}{
						"arch": "amd64",
						"auth": tc.Auth,
					},
				}, {
					Address:   tc.ResourceType + ".dev",
					Type:      tc.ResourceType,
					Name:      "dev",
					Mode:      tfjson.ManagedResourceMode,
					DependsOn: []string{"coder_agent.dev"},
					AttributeValues: map[string]interface{}{
						tc.InstanceIDKey: instanceID,
					},
				}},
				// This is manually created to join the edges.
			}}, `digraph {
	compound = "true"
	newrank = "true"
	subgraph "root" {
		"[root] coder_agent.dev" [label = "coder_agent.dev", shape = "box"]
		"[root] `+tc.ResourceType+`.dev" [label = "`+tc.ResourceType+`.dev", shape = "box"]
		"[root] `+tc.ResourceType+`.dev" -> "[root] coder_agent.dev"
	}
}
`)
			require.NoError(t, err)
			require.Len(t, state.Resources, 1)
			require.Len(t, state.Resources[0].Agents, 1)
			require.Equal(t, state.Resources[0].Agents[0].GetInstanceId(), instanceID)
		})
	}
}

// sortResource ensures resources appear in a consistent ordering
// to prevent tests from flaking.
func sortResources(resources []*proto.Resource) {
	sort.Slice(resources, func(i, j int) bool {
		if resources[i].Name != resources[j].Name {
			return resources[i].Name < resources[j].Name
		}
		return resources[i].Type < resources[j].Type
	})
	for _, resource := range resources {
		for _, agent := range resource.Agents {
			sort.Slice(agent.Apps, func(i, j int) bool {
				return agent.Apps[i].Slug < agent.Apps[j].Slug
			})
		}
		sort.Slice(resource.Agents, func(i, j int) bool {
			return resource.Agents[i].Name < resource.Agents[j].Name
		})
	}
}
