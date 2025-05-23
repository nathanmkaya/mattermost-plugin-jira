// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"path/filepath"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin/plugintest"
	"github.com/mattermost/mattermost/server/public/pluginapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-plugin-jira/server/enterprise"
)

func TestInstallInstance(t *testing.T) {
	trueValue := true
	p := &Plugin{}

	for name, tc := range map[string]struct {
		license      *model.License
		numInstances int
		expectError  bool
		devEnabled   bool
	}{
		"0 preinstalled, valid license": {
			numInstances: 0,
			expectError:  false,
			license: &model.License{
				SkuShortName: "professional",
			},
		},
		"0 preinstalled, nil license": {
			numInstances: 0,
			expectError:  false,
			license:      nil,
		},
		"1 preinstalled, professional license": {
			numInstances: 1,
			expectError:  false,
			license: &model.License{
				SkuShortName: "professional",
			},
		},
		"1 preinstalled, Enterprise Advanced license": {
			numInstances: 1,
			expectError:  false,
			license: &model.License{
				SkuShortName: "advanced",
			},
		},
		"1 preinstalled, enterprise license": {
			numInstances: 1,
			expectError:  false,
			license: &model.License{
				SkuShortName: "enterprise",
			},
		},
		"1 preinstalled, cloud starter license. should have error": {
			numInstances: 1,
			expectError:  true,
			license: &model.License{
				SkuShortName: "starter",
			},
		},
		"1 preinstalled, dev mode": {
			numInstances: 1,
			expectError:  false,
			license:      nil,
			devEnabled:   true,
		},
		"1 preinstalled  nil license": {
			numInstances: 1,
			expectError:  true,
			license:      nil,
		},
	} {
		t.Run(name, func(t *testing.T) {
			api := &plugintest.API{}

			p.SetAPI(api)
			p.client = pluginapi.NewClient(api, p.Driver)
			p.enterpriseChecker = enterprise.NewEnterpriseChecker(api)
			p.instanceStore = p.getMockInstanceStoreKV(tc.numInstances)

			conf := &model.Config{}
			if tc.devEnabled {
				conf.ServiceSettings.EnableDeveloper = &trueValue
				conf.ServiceSettings.EnableTesting = &trueValue
			}

			api.On("KVGet", mock.Anything).Return(mock.Anything, nil)
			api.On("GetLicense").Return(tc.license)
			api.On("GetConfig").Return(conf)
			api.On("UnregisterCommand", mock.Anything, mock.Anything).Return(nil)
			api.On("RegisterCommand", mock.Anything, mock.Anything).Return(nil)
			api.On("PublishWebSocketEvent", mock.Anything, mock.Anything, mock.Anything)

			path, err := filepath.Abs("..")
			require.Nil(t, err)
			api.On("GetBundlePath").Return(path, nil)

			testInstance0 := &testInstance{
				InstanceCommon: InstanceCommon{
					InstanceID: mockInstance3URL,
					IsV2Legacy: true,
					Type:       "testInstanceType",
				},
			}

			err = p.InstallInstance(testInstance0)
			if tc.expectError {
				assert.NotNil(t, err)
				expected := "You need a valid Mattermost Professional, Enterprise or Enterprise Advanced License to install multiple Jira instances."
				assert.Equal(t, expected, err.Error())
			} else {
				assert.Nil(t, err)
			}
		})
	}
}
