// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package omni provides the Omni API integration of the server.
package omni

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/omni/client/pkg/client"
	omniconstants "github.com/siderolabs/omni/client/pkg/constants"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	"go.uber.org/zap"
)

// ServiceAccountKeyEnvVar is the environment variable that contains the Omni service account key.
const ServiceAccountKeyEnvVar = "OMNI_SERVICE_ACCOUNT_KEY"

// Options defines the options for connecting to Omni.
type Options struct {
	APIEndpoint              string
	APIInsecureSkipTLSVerify bool
}

// Enabled returns whether an Omni connection is requested.
func (o Options) Enabled() bool {
	return o.APIEndpoint != ""
}

// ConnectionOptions contains the parameters machines need to connect to Omni over SideroLink.
type ConnectionOptions struct {
	SiderolinkAPIURL string
	JoinToken        string

	EventsPort  int
	KmsgLogPort int
}

// NewClient creates a new Omni API client, authenticated with the service account key read from ServiceAccountKeyEnvVar.
//
// The caller is responsible for closing the client.
func NewClient(options Options) (*client.Client, error) {
	if options.APIEndpoint == "" {
		return nil, errors.New("omni api endpoint is not set")
	}

	serviceAccountKey := os.Getenv(ServiceAccountKeyEnvVar)
	if serviceAccountKey == "" {
		return nil, fmt.Errorf("environment variable %q is not set", ServiceAccountKeyEnvVar)
	}

	return client.New(options.APIEndpoint,
		client.WithServiceAccount(serviceAccountKey),
		client.WithInsecureSkipTLSVerify(options.APIInsecureSkipTLSVerify),
	)
}

// GetConnectionOptions reads the SideroLink connection parameters from Omni.
func GetConnectionOptions(ctx context.Context, st state.State, logger *zap.Logger) (ConnectionOptions, error) {
	//nolint:staticcheck
	connectionParams, err := safe.StateGetByID[*siderolink.ConnectionParams](ctx, st, siderolink.ConfigID)
	if err != nil {
		return ConnectionOptions{}, fmt.Errorf("failed to get Omni connection params: %w", err)
	}

	logger.Info(
		"fetched Omni connection parameters",
		zap.String("api_endpoint", connectionParams.TypedSpec().Value.ApiEndpoint),
		zap.Int("events_port", int(connectionParams.TypedSpec().Value.EventsPort)),
		zap.Int("kmsg_log_port", int(connectionParams.TypedSpec().Value.LogsPort)),
	)

	return ConnectionOptions{
		SiderolinkAPIURL: connectionParams.TypedSpec().Value.ApiEndpoint,
		JoinToken:        connectionParams.TypedSpec().Value.JoinToken,
		EventsPort:       int(connectionParams.TypedSpec().Value.EventsPort),
		KmsgLogPort:      int(connectionParams.TypedSpec().Value.LogsPort),
	}, nil
}

// DefaultTalosVersion returns the Talos version Omni uses as its default, which Omni marks with the omni.LabelDefaultVersion label.
//
// An Omni that labels no version falls back to the default of the Omni client library.
func DefaultTalosVersion(ctx context.Context, st state.State) (string, error) {
	versions, err := safe.StateListAll[*omni.TalosVersion](ctx, st, state.WithLabelQuery(resource.LabelExists(omni.LabelDefaultVersion)))
	if err != nil {
		return "", fmt.Errorf("failed to get the default Talos version from Omni: %w", err)
	}

	if versions.Len() > 0 {
		return versions.Get(0).TypedSpec().Value.Version, nil
	}

	return omniconstants.DefaultTalosVersion, nil
}
