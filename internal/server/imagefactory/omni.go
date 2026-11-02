// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package imagefactory

import (
	"context"
	"fmt"
	"maps"
	"net/http"
	"slices"
	"strings"

	"github.com/siderolabs/omni/client/api/omni/management"
	"github.com/siderolabs/omni/client/pkg/client"
	omnifactory "github.com/siderolabs/omni/client/pkg/imagefactory"
	"github.com/siderolabs/omni/client/pkg/infra"
	"github.com/siderolabs/omni/client/pkg/infra/provision"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/booter/internal/server/omni"
)

// OmniClient resolves boot assets through Omni.
//
// Omni ensures the schematic on the image factory it is configured with, and returns an iPXE URL which carries
// whatever authentication that factory needs (e.g., a short-lived download token for an Enterprise Image Factory),
// so no factory address or credentials live here.
type OmniClient struct {
	omniClient        *client.Client
	logger            *zap.Logger
	secureBootEnabled bool
}

// NewOmniClient creates a new image factory client which goes through Omni.
func NewOmniClient(omniClient *client.Client, secureBootEnabled bool, logger *zap.Logger) *OmniClient {
	return &OmniClient{
		omniClient:        omniClient,
		secureBootEnabled: secureBootEnabled,
		logger:            logger,
	}
}

// IPXEURL ensures the schematic of the request exists on the image factory Omni is configured with, unless the request names
// an existing one, and returns a self-contained iPXE URL of its boot assets.
//
// The URL is resolved on every call, as it might embed a download token which expires.
func (c *OmniClient) IPXEURL(ctx context.Context, request IPXERequest) (string, error) {
	if err := request.validate(); err != nil {
		return "", err
	}

	logger := c.logger.With(zap.String("talos_version", request.TalosVersion), zap.String("arch", request.Arch))

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	spec := provision.MediaSpec{
		Kind:         omnifactory.InstallationMediaKindPXE,
		Platform:     constants.PlatformMetal,
		Architecture: request.Arch,
		SecureBoot:   c.secureBootEnabled,
		// iPXE fetches the script, and the factory forwards the credential into the asset URLs of the script it returns
		StandaloneURL: true,
	}

	var (
		media omnifactory.InstallationMedia
		err   error
	)

	if request.SchematicID != "" {
		media, err = c.resolveExistingSchematic(ctx, request, spec)
	} else {
		sch := request.schematic()

		if marshaled, marshalErr := sch.Marshal(); marshalErr == nil {
			logger.Debug("generated schematic", zap.String("schematic", string(marshaled)))
		}

		media, err = infra.EnsureInstallationMedia(ctx, c.omniClient, request.TalosVersion, sch, spec)
	}

	if err != nil {
		return "", fmt.Errorf("failed to resolve the iPXE installation media through Omni: %w", err)
	}

	if len(media.Headers) > 0 {
		return "", fmt.Errorf("omni returned an iPXE URL which requires headers (%s), which iPXE cannot send", strings.Join(slices.Sorted(maps.Keys(media.Headers)), ", "))
	}

	logger.Debug("resolved iPXE URL through Omni",
		zap.String("schematic_id", media.SchematicID),
		zap.String("image_factory_host", media.ImageFactoryHost),
		zap.Time("expires_at", media.ExpiresAt))

	return media.URL, nil
}

// resolveExistingSchematic resolves the installation media of a schematic which already exists on the image factory.
//
// The infra package only resolves media for a schematic it ensures itself, so this goes to the same management API directly,
// with the same fallback for an Omni which predates it.
func (c *OmniClient) resolveExistingSchematic(ctx context.Context, request IPXERequest, spec provision.MediaSpec) (omnifactory.InstallationMedia, error) {
	resp, err := c.omniClient.Management().GetInstallationMediaURL(ctx, &management.InstallationMediaURLRequest{
		TalosVersion:          request.TalosVersion,
		SchematicId:           request.SchematicID,
		StandaloneUrl:         spec.StandaloneURL,
		InstallationMediaKind: management.InstallationMediaURLRequest_INSTALLATION_MEDIA_KIND_PXE,
		Platform:              spec.Platform,
		Architecture:          spec.Architecture,
		SecureBoot:            spec.SecureBoot,
	})
	if err != nil {
		if status.Code(err) != codes.Unimplemented {
			return omnifactory.InstallationMedia{}, fmt.Errorf("failed to get the installation media URL from Omni: %w", err)
		}

		return omnifactory.ResolveInstallationMedia(ctx, c.omniClient.Omni().State(), request.TalosVersion, spec.MediaSpec, request.SchematicID, spec.StandaloneURL)
	}

	var headers http.Header

	for name, value := range resp.Headers {
		if headers == nil {
			headers = make(http.Header, len(resp.Headers))
		}

		headers.Set(name, value)
	}

	return omnifactory.InstallationMedia{
		URL:              resp.Url,
		SchematicID:      request.SchematicID,
		ImageFactoryHost: resp.ImageFactoryHost,
		Headers:          headers,
	}, nil
}

// DefaultTalosVersion returns the Talos version Omni uses as its default.
func (c *OmniClient) DefaultTalosVersion(ctx context.Context) (string, error) {
	return omni.DefaultTalosVersion(ctx, c.omniClient.Omni().State())
}
