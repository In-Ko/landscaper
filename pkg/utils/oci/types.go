// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors.
//
// SPDX-License-Identifier: Apache-2.0

package oci

import (
	"context"
	"io"
	"net/http"

	"github.com/containerd/containerd/remotes"
	ocispecv1 "github.com/opencontainers/image-spec/specs-go/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/gardener/landscaper/pkg/apis/config"
	"github.com/gardener/landscaper/pkg/utils/oci/cache"
	"github.com/gardener/landscaper/pkg/utils/oci/credentials"
)

type Client interface {
	// GetManifest returns the ocispec Manifest for a reference
	GetManifest(ctx context.Context, ref string) (*ocispecv1.Manifest, error)

	// Fetch fetches the blob for the given ocispec Descriptor.
	Fetch(ctx context.Context, ref string, desc ocispecv1.Descriptor, writer io.Writer) error

	// PushManifest uploads the given ocispec Descriptor to the given ref.
	PushManifest(ctx context.Context, ref string, manifest *ocispecv1.Manifest) error
}

// Resolver is a interface that should return a new resolver if called.
type Resolver interface {
	// Resolver returns a new authenticated resolver.
	Resolver(ctx context.Context, client *http.Client, plainHTTP bool) (remotes.Resolver, error)
}

// Options contains all client options to configure the oci client.
type Options struct {
	// Paths configures local paths to search for docker configuration files
	Paths []string

	// Resolver sets the used resolver.
	// A default resolver will be created if not given.
	Resolver Resolver

	// CacheConfig contains the cache configuration.
	// Tis configuration will automatically create a new cache based on that configuration.
	// This cache can be overwritten with the Cache property.
	CacheConfig *config.OCICacheConfiguration

	// Cache is the oci cache to be used by the client
	Cache cache.Cache

	// CustomMediaTypes defines the custom known media types
	CustomMediaTypes sets.String
}

// Option is the interface to specify different cache options
type Option interface {
	ApplyOption(options *Options)
}

// ApplyOptions applies the given list options on these options,
// and then returns itself (for convenient chaining).
func (o *Options) ApplyOptions(opts []Option) *Options {
	for _, opt := range opts {
		if opt != nil {
			opt.ApplyOption(o)
		}
	}
	return o
}

// WithCache configures a cache for the oci client
type WithCache struct {
	cache.Cache
}

func (c WithCache) ApplyOption(options *Options) {
	options.Cache = c.Cache
}

// WithKeyring configures the resolver to use the given oci keyring
type WithKeyring struct {
	credentials.OCIKeyring
}

func (c *WithKeyring) ApplyOption(options *Options) {
	options.Resolver = c.OCIKeyring
}

// WithResolver configures a resolver for the oci client
type WithResolver struct {
	Resolver
}

func (c WithResolver) ApplyOption(options *Options) {
	options.Resolver = c.Resolver
}

// WithConfiguration applies external oci configuration as internal options.
func WithConfiguration(cfg *config.OCIConfiguration) *WithConfigurationStruct {
	if cfg == nil {
		return nil
	}
	wc := WithConfigurationStruct(*cfg)
	return &wc
}

// WithKnownMediaType adds a known media types to the client
type WithKnownMediaType string

func (c WithKnownMediaType) ApplyOption(options *Options) {
	if options.CustomMediaTypes == nil {
		options.CustomMediaTypes = sets.NewString(string(c))
		return
	}

	options.CustomMediaTypes.Insert(string(c))
}

// WithConfiguration applies external oci configuration as internal options.
type WithConfigurationStruct config.OCIConfiguration

func (c *WithConfigurationStruct) ApplyOption(options *Options) {
	if c == nil {
		return
	}
	if len(c.ConfigFiles) != 0 {
		options.Paths = c.ConfigFiles
	}
	if c.Cache != nil {
		options.CacheConfig = c.Cache
	}
}
