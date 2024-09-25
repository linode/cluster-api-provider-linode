/*
Copyright 2023 Akamai Technologies, Inc.

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

package scope

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/linode/linodego"
)

type ImageCache struct {
	lastUpdate time.Time
	TTL        time.Duration
	image      *linodego.Image
	ClientConfig ClientConfig
}

// LinodeCache
type LinodeCache struct {
	mu           sync.RWMutex
	regions      map[string]*linodego.Region
	ImageCache   map[string]*ImageCache
	lastUpdate   time.Time
	TTL          time.Duration
	ClientConfig ClientConfig
}

func (lc *LinodeCache) refreshRegions(ctx context.Context) error {
	lc.mu.Lock()
	defer lc.mu.Unlock()

	if time.Since(lc.lastUpdate) < lc.TTL {
		return nil
	}

	client, err := CreateLinodeClient(lc.ClientConfig,
		WithRetryCount(0),
	)
	if err != nil {
		return err
	}
	lc.regions = make(map[string]*linodego.Region, 0)
	regions, err := client.ListRegions(ctx, &linodego.ListOptions{})
	if err != nil {
		return err
	}

	for _, region := range regions {
		lc.regions[region.ID] = &region
	}
	lc.lastUpdate = time.Now()
	return nil
}

func (lc *LinodeCache) GetRegion(ctx context.Context, regionName string) (*linodego.Region, error) {
	if err := lc.refreshRegions(ctx); err != nil {
		return nil, err
	}

	return lc.getRegionFromName(regionName)
}

func (lc *LinodeCache) getRegionFromName(regionName string) (*linodego.Region, error) {
	lc.mu.RLock()
	defer lc.mu.RUnlock()

	region, ok := lc.regions[regionName]
	if !ok {
		return nil, fmt.Errorf("failed to find region with name %s", regionName)
	}
	return region, nil
}

func (lc *LinodeCache) refreshImage(ctx context.Context, imageName string) error {
	lc.mu.Lock()
	defer lc.mu.Unlock()

	imageObj, ok := lc.ImageCache[imageName]
	if !ok || (time.Since(imageObj.lastUpdate) > imageObj.TTL) {

		client, err := CreateLinodeClient(lc.ClientConfig,
			WithRetryCount(0),
		)
		if err != nil {
			return err
		}

		image, err := client.GetImage(ctx, imageName)
		if err != nil {
			return err
		}
		imageObj = &ImageCache{TTL: 15 * time.Minute, image: image, lastUpdate: time.Now()}
		lc.ImageCache[imageName] = imageObj
	}
	return nil
}

func (lc *LinodeCache) GetImage(ctx context.Context, imageName string) (*linodego.Image, error) {
	if err := lc.refreshImage(ctx, imageName); err != nil {
		return nil, err
	}

	return lc.getImageFromName(imageName)
}

func (lc *LinodeCache) getImageFromName(imageName string) (*linodego.Image, error) {
	lc.mu.RLock()
	defer lc.mu.RUnlock()

	imageObj, ok := lc.ImageCache[imageName]
	if !ok {
		return nil, fmt.Errorf("failed to find image with name %s", imageName)
	}
	return imageObj.image, nil
}
