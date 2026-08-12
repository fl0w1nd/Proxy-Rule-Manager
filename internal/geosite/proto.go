package geosite

import (
	"fmt"

	"google.golang.org/protobuf/proto"
)

// decodeGeoSiteList parses a v2fly-style GeoSiteList protobuf message using
// generated code from geosite.proto, replacing the former hand-written
// protowire decoder.
func decodeGeoSiteList(buf []byte) (*GeoSiteList, error) {
	var siteList GeoSiteList
	if err := proto.Unmarshal(buf, &siteList); err != nil {
		return nil, fmt.Errorf("decode geosite list: %w", err)
	}
	return &siteList, nil
}
