// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package imagefactory

import (
	"context"
	"errors"
	"fmt"

	"github.com/blang/semver/v4"
	"github.com/siderolabs/image-factory/pkg/client"
	"go.uber.org/zap"
)

// Client is an image factory client which talks to an unauthenticated image factory directly.
type Client struct {
	factoryClient     *client.Client
	logger            *zap.Logger
	pxeBaseURL        string
	secureBootEnabled bool
}

// NewClient creates a new image factory client.
func NewClient(baseURL, pxeBaseURL string, secureBootEnabled bool, logger *zap.Logger) (*Client, error) {
	factoryClient, err := client.New(baseURL)
	if err != nil {
		return nil, err
	}

	return &Client{
		pxeBaseURL:        pxeBaseURL,
		factoryClient:     factoryClient,
		secureBootEnabled: secureBootEnabled,
		logger:            logger,
	}, nil
}

// IPXEURL ensures the schematic of the request exists on the image factory, unless the request names an existing one,
// and returns the iPXE URL of its boot assets.
func (c *Client) IPXEURL(ctx context.Context, request IPXERequest) (string, error) {
	if err := request.validate(); err != nil {
		return "", err
	}

	schematicID := request.SchematicID

	if schematicID == "" {
		var err error

		if schematicID, err = c.ensureSchematic(ctx, request); err != nil {
			return "", err
		}
	}

	ipxeURL := fmt.Sprintf("%s/pxe/%s/%s/metal-%s", c.pxeBaseURL, schematicID, request.TalosVersion, request.Arch)

	if c.secureBootEnabled {
		ipxeURL += "-secureboot"
	}

	return ipxeURL, nil
}

func (c *Client) ensureSchematic(ctx context.Context, request IPXERequest) (string, error) {
	logger := c.logger.With(zap.Strings("extensions", request.Extensions), zap.Strings("extra_kernel_args", request.ExtraKernelArgs))

	logger.Debug("ensure schematic")

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	sch := request.schematic()

	marshaled, err := sch.Marshal()
	if err != nil {
		return "", fmt.Errorf("failed to marshal schematic: %w", err)
	}

	logger.Debug("generated schematic", zap.String("schematic", string(marshaled)))

	schematicID, _, err := c.factoryClient.SchematicCreate(ctx, sch)
	if err != nil {
		return "", fmt.Errorf("failed to create schematic: %w", err)
	}

	return schematicID, nil
}

// DefaultTalosVersion returns the latest stable Talos version from the image factory.
func (c *Client) DefaultTalosVersion(ctx context.Context) (string, error) {
	versions, err := c.factoryClient.Versions(ctx)
	if err != nil {
		return "", err
	}

	var latestStable *semver.Version

	for _, v := range versions {
		sv, err := semver.ParseTolerant(v)
		if err != nil {
			c.logger.Warn("failed to parse version", zap.String("version", v), zap.Error(err))

			continue
		}

		// Skip pre-releases
		if len(sv.Pre) > 0 {
			continue
		}

		if latestStable == nil || sv.GT(*latestStable) {
			latestStable = &sv
		}
	}

	if latestStable == nil {
		return "", errors.New("no stable versions found")
	}

	return latestStable.String(), nil
}
