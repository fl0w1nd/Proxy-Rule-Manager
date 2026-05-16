package geosite

import (
	"fmt"
	"strings"

	"google.golang.org/protobuf/encoding/protowire"
)

// Domain mirrors the .proto Domain message.
type protoDomain struct {
	Type      int32
	Value     string
	Attribute []string
}

// GeoSite mirrors the GeoSite message.
type protoGeoSite struct {
	CountryCode string
	Domains     []protoDomain
}

// decodeGeoSiteList parses a v2fly-style GeoSiteList protobuf message.
func decodeGeoSiteList(buf []byte) ([]protoGeoSite, error) {
	var out []protoGeoSite
	for len(buf) > 0 {
		num, typ, n := protowire.ConsumeTag(buf)
		if n < 0 {
			return nil, fmt.Errorf("bad tag at offset")
		}
		buf = buf[n:]
		if num == 1 && typ == protowire.BytesType {
			payload, n := protowire.ConsumeBytes(buf)
			if n < 0 {
				return nil, fmt.Errorf("bad bytes for GeoSite")
			}
			buf = buf[n:]
			site, err := decodeGeoSite(payload)
			if err != nil {
				return nil, err
			}
			out = append(out, site)
			continue
		}
		size := protowire.ConsumeFieldValue(num, typ, buf)
		if size < 0 {
			return nil, fmt.Errorf("bad field skip")
		}
		buf = buf[size:]
	}
	return out, nil
}

func decodeGeoSite(buf []byte) (protoGeoSite, error) {
	var site protoGeoSite
	for len(buf) > 0 {
		num, typ, n := protowire.ConsumeTag(buf)
		if n < 0 {
			return site, fmt.Errorf("bad tag in GeoSite")
		}
		buf = buf[n:]
		switch {
		case num == 1 && typ == protowire.BytesType:
			payload, n := protowire.ConsumeString(buf)
			if n < 0 {
				return site, fmt.Errorf("bad country_code")
			}
			buf = buf[n:]
			site.CountryCode = strings.TrimSpace(payload)
		case num == 2 && typ == protowire.BytesType:
			payload, n := protowire.ConsumeBytes(buf)
			if n < 0 {
				return site, fmt.Errorf("bad Domain bytes")
			}
			buf = buf[n:]
			d, err := decodeDomain(payload)
			if err != nil {
				return site, err
			}
			site.Domains = append(site.Domains, d)
		default:
			size := protowire.ConsumeFieldValue(num, typ, buf)
			if size < 0 {
				return site, fmt.Errorf("bad GeoSite field")
			}
			buf = buf[size:]
		}
	}
	return site, nil
}

func decodeDomain(buf []byte) (protoDomain, error) {
	var d protoDomain
	for len(buf) > 0 {
		num, typ, n := protowire.ConsumeTag(buf)
		if n < 0 {
			return d, fmt.Errorf("bad tag in Domain")
		}
		buf = buf[n:]
		switch {
		case num == 1 && typ == protowire.VarintType:
			v, n := protowire.ConsumeVarint(buf)
			if n < 0 {
				return d, fmt.Errorf("bad domain type")
			}
			buf = buf[n:]
			d.Type = int32(v)
		case num == 2 && typ == protowire.BytesType:
			v, n := protowire.ConsumeString(buf)
			if n < 0 {
				return d, fmt.Errorf("bad domain value")
			}
			buf = buf[n:]
			d.Value = v
		case num == 3 && typ == protowire.BytesType:
			v, n := protowire.ConsumeBytes(buf)
			if n < 0 {
				return d, fmt.Errorf("bad attribute bytes")
			}
			buf = buf[n:]
			key, err := decodeAttributeKey(v)
			if err != nil {
				return d, err
			}
			if key != "" {
				d.Attribute = append(d.Attribute, key)
			}
		default:
			size := protowire.ConsumeFieldValue(num, typ, buf)
			if size < 0 {
				return d, fmt.Errorf("bad Domain field")
			}
			buf = buf[size:]
		}
	}
	return d, nil
}

func decodeAttributeKey(buf []byte) (string, error) {
	for len(buf) > 0 {
		num, typ, n := protowire.ConsumeTag(buf)
		if n < 0 {
			return "", fmt.Errorf("bad tag in Attribute")
		}
		buf = buf[n:]
		if num == 1 && typ == protowire.BytesType {
			v, n := protowire.ConsumeString(buf)
			if n < 0 {
				return "", fmt.Errorf("bad attribute key")
			}
			return v, nil
		}
		size := protowire.ConsumeFieldValue(num, typ, buf)
		if size < 0 {
			return "", fmt.Errorf("bad attribute field")
		}
		buf = buf[size:]
	}
	return "", nil
}
