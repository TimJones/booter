// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package imagefactory_test

import (
	"context"
	"net"
	"sync"
	"testing"

	"github.com/siderolabs/image-factory/pkg/schematic"
	"github.com/siderolabs/omni/client/api/omni/management"
	"github.com/siderolabs/omni/client/pkg/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"google.golang.org/grpc"
	_ "google.golang.org/grpc/encoding/gzip" // the Omni client compresses requests with gzip

	"github.com/siderolabs/booter/internal/server/imagefactory"
)

const (
	testSchematicID = "376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba"
	testMediaURL    = "https://enterprise.factory.example/pxe/" + testSchematicID + "/v1.14.5/metal-arm64-secureboot?token=download-token"
)

// fakeManagement is an Omni management API which records the schematic and installation media requests it receives.
type fakeManagement struct {
	management.UnimplementedManagementServiceServer

	mediaResponse *management.InstallationMediaURLResponse

	schematicRequests []*management.CreateSchematicFromRawRequest
	mediaRequests     []*management.InstallationMediaURLRequest
	mu                sync.Mutex
}

func (f *fakeManagement) CreateSchematicFromRaw(_ context.Context, req *management.CreateSchematicFromRawRequest) (*management.CreateSchematicResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.schematicRequests = append(f.schematicRequests, req)

	return &management.CreateSchematicResponse{SchematicId: testSchematicID}, nil
}

func (f *fakeManagement) GetInstallationMediaURL(_ context.Context, req *management.InstallationMediaURLRequest) (*management.InstallationMediaURLResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.mediaRequests = append(f.mediaRequests, req)

	return f.mediaResponse, nil
}

func startFakeOmni(t *testing.T, fake *fakeManagement) *client.Client {
	t.Helper()

	listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)

	server := grpc.NewServer()
	management.RegisterManagementServiceServer(server, fake)

	go server.Serve(listener) //nolint:errcheck

	t.Cleanup(server.Stop)

	omniClient, err := client.New("grpc://" + listener.Addr().String())
	require.NoError(t, err)

	t.Cleanup(func() { omniClient.Close() }) //nolint:errcheck

	return omniClient
}

func TestOmniClientEnsuresSchematic(t *testing.T) {
	t.Parallel()

	fake := &fakeManagement{
		mediaResponse: &management.InstallationMediaURLResponse{Url: testMediaURL, ImageFactoryHost: "enterprise.factory.example"},
	}

	factoryClient := imagefactory.NewOmniClient(startFakeOmni(t, fake), true, zaptest.NewLogger(t))

	ipxeURL, err := factoryClient.IPXEURL(t.Context(), imagefactory.IPXERequest{
		TalosVersion:    "v1.14.5",
		Arch:            "arm64",
		Extensions:      []string{"siderolabs/iscsi-tools"},
		ExtraKernelArgs: []string{"talos.config=http://192.168.1.100:50084/config", "console=ttyAMA0"},
	})
	require.NoError(t, err)

	// the URL is used as is, so the download token Omni put into it reaches iPXE
	assert.Equal(t, testMediaURL, ipxeURL)

	require.Len(t, fake.schematicRequests, 1)
	assert.Equal(t, "v1.14.5", fake.schematicRequests[0].TalosVersion)

	sch, err := schematic.Unmarshal(fake.schematicRequests[0].RawSchematic)
	require.NoError(t, err)
	assert.Equal(t, []string{"siderolabs/iscsi-tools"}, sch.Customization.SystemExtensions.OfficialExtensions)
	assert.Equal(t, []string{"talos.config=http://192.168.1.100:50084/config", "console=ttyAMA0"}, sch.Customization.ExtraKernelArgs)

	require.Len(t, fake.mediaRequests, 1)
	assertPXEMediaRequest(t, fake.mediaRequests[0])
}

func TestOmniClientExistingSchematic(t *testing.T) {
	t.Parallel()

	fake := &fakeManagement{
		mediaResponse: &management.InstallationMediaURLResponse{Url: testMediaURL},
	}

	factoryClient := imagefactory.NewOmniClient(startFakeOmni(t, fake), true, zaptest.NewLogger(t))

	ipxeURL, err := factoryClient.IPXEURL(t.Context(), imagefactory.IPXERequest{
		SchematicID:  testSchematicID,
		TalosVersion: "v1.14.5",
		Arch:         "arm64",
	})
	require.NoError(t, err)
	assert.Equal(t, testMediaURL, ipxeURL)

	assert.Empty(t, fake.schematicRequests, "an existing schematic must not be created")

	require.Len(t, fake.mediaRequests, 1)
	assertPXEMediaRequest(t, fake.mediaRequests[0])
}

func TestOmniClientRejectsHeaders(t *testing.T) {
	t.Parallel()

	fake := &fakeManagement{
		mediaResponse: &management.InstallationMediaURLResponse{
			Url:     testMediaURL,
			Headers: map[string]string{"Authorization": "Basic Zm9vOmJhcg=="},
		},
	}

	factoryClient := imagefactory.NewOmniClient(startFakeOmni(t, fake), true, zaptest.NewLogger(t))

	for _, schematicID := range []string{"", testSchematicID} {
		_, err := factoryClient.IPXEURL(t.Context(), imagefactory.IPXERequest{
			SchematicID:  schematicID,
			TalosVersion: "v1.14.5",
			Arch:         "arm64",
		})
		require.ErrorContains(t, err, "which iPXE cannot send")
	}
}

func assertPXEMediaRequest(t *testing.T, req *management.InstallationMediaURLRequest) {
	t.Helper()

	assert.Equal(t, management.InstallationMediaURLRequest_INSTALLATION_MEDIA_KIND_PXE, req.InstallationMediaKind)
	assert.Equal(t, testSchematicID, req.SchematicId)
	assert.Equal(t, "v1.14.5", req.TalosVersion)
	assert.Equal(t, "metal", req.Platform)
	assert.Equal(t, "arm64", req.Architecture)
	assert.True(t, req.SecureBoot)
	assert.True(t, req.StandaloneUrl, "iPXE cannot send headers, so the URL must carry the credentials")
}
